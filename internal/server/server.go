package server

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
	"github.com/umbracle/viewpoint/internal/components"
	"github.com/umbracle/viewpoint/internal/docker"
	"github.com/umbracle/viewpoint/internal/genesis"
	"github.com/umbracle/viewpoint/internal/server/proto"
	"github.com/umbracle/viewpoint/internal/spec"
	"google.golang.org/grpc"
)

type Server struct {
	proto.UnimplementedE2EServiceServer
	config *Config
	logger hclog.Logger

	eth1HttpAddr   string
	depositHandler *depositHandler

	// runtime to deploy containers
	docker *docker.Docker

	lock        sync.Mutex
	fileLogger  *fileLogger
	nodes       []spec.Node
	bootnodeENR string

	// genesis data
	genesisSSZ      []byte
	genesisAccounts []*proto.Account
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	docker, err := docker.NewDocker()
	if err != nil {
		return nil, err
	}

	eth1, err := docker.Deploy(components.NewEth1Server())
	if err != nil {
		return nil, err
	}
	bootnodeSpec := components.NewBootnode()
	bootnode, err := docker.Deploy(bootnodeSpec.Spec)
	if err != nil {
		return nil, err
	}

	// get latest block
	provider, err := jsonrpc.NewClient(eth1.GetAddr(proto.NodePortEth1Http))
	if err != nil {
		return nil, err
	}
	block, err := provider.Eth().GetBlockByNumber(ethgo.Latest, true)
	if err != nil {
		return nil, err
	}

	accounts := proto.NewAccounts(config.Spec.GenesisValidatorCount)

	genesisInit := config.Spec.MinGenesisTime

	state, err := genesis.GenerateGenesis(block, int64(genesisInit), accounts)
	if err != nil {
		return nil, err
	}
	genesisSSZ, err := state.MarshalSSZ()
	if err != nil {
		return nil, err
	}

	depositHandler, err := newDepositHandler(eth1.GetAddr(proto.NodePortEth1Http))
	if err != nil {
		return nil, err
	}

	dataPath := "e2e-" + config.Name

	// create a folder to store data
	if err := os.Mkdir(dataPath, 0755); err != nil {
		return nil, err
	}

	logger.Info("eth1 server deployed", "addr", eth1.GetAddr(proto.NodePortEth1Http))
	logger.Info("deposit contract deployed", "addr", depositHandler.deposit.String())

	config.Spec.DepositContract = depositHandler.deposit.String()

	srv := &Server{
		config:       config,
		logger:       logger,
		eth1HttpAddr: eth1.GetAddr(proto.NodePortEth1Http),
		docker:       docker,
		nodes: []spec.Node{
			eth1,
			bootnode,
		},
		genesisAccounts: accounts,
		fileLogger:      &fileLogger{path: dataPath},
		bootnodeENR:     bootnodeSpec.Enr,
		depositHandler:  depositHandler,
		genesisSSZ:      genesisSSZ,
	}

	if err := srv.writeFile("spec.yaml", config.Spec.buildConfig()); err != nil {
		return nil, err
	}
	if err := srv.writeFile("genesis.ssz", genesisSSZ); err != nil {
		return nil, err
	}

	grpcServer := grpc.NewServer()
	proto.RegisterE2EServiceServer(grpcServer, srv)

	grpcAddr := "localhost:5555"
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("failed to serve grpc server", "err", err)
		}
	}()
	logger.Info("GRPC Server started", "addr", grpcAddr)

	return srv, nil
}

func (s *Server) writeFile(path string, content []byte) error {
	localPath := filepath.Join("e2e-"+s.config.Name, path)

	parentDir := filepath.Dir(localPath)
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return err
	}
	if err := ioutil.WriteFile(localPath, []byte(content), 0644); err != nil {
		return err
	}
	return nil
}

func (s *Server) Stop() {
	// stop all servers
	for _, node := range s.nodes {
		if err := node.Stop(); err != nil {
			s.logger.Error("failed to stop node", "id", "x", "err", err)
		}
	}
	if err := s.fileLogger.Close(); err != nil {
		s.logger.Error("failed to close file logger", "err", err.Error())
	}
}

func (s *Server) filterLocked(cond func(opts *spec.Spec) bool) []spec.Node {
	res := []spec.Node{}
	for _, i := range s.nodes {
		if cond(i.Spec()) {
			res = append(res, i)
		}
	}
	return res
}

func (s *Server) NodeDeploy(ctx context.Context, req *proto.NodeDeployRequest) (*proto.NodeDeployResponse, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	numOfNodes := func(typ proto.NodeType) int {
		nodes := s.filterLocked(func(spec *spec.Spec) bool {
			return spec.HasLabel(proto.NodeTypeLabel, typ.String())
		})
		return len(nodes)
	}

	deployNode := func(name string, spec *spec.Spec) (*proto.Node, error) {
		fLogger, err := s.fileLogger.Create(name)
		if err != nil {
			return nil, err
		}
		spec.WithName(name).
			WithOutput(fLogger)

		node, err := s.docker.Deploy(spec)
		if err != nil {
			return nil, err
		}
		s.nodes = append(s.nodes, node)

		stub, err := specNodeToNode(node)
		if err != nil {
			return nil, err
		}
		return stub, nil
	}

	var node *proto.Node

	if _, ok := req.NodeType.(*proto.NodeDeployRequest_Beacon_); ok {
		name := fmt.Sprintf("beacon-%d-%s", numOfNodes(proto.NodeType_Beacon), req.NodeClient.String())

		bCfg := &proto.BeaconConfig{
			Spec:       s.config.Spec.buildConfig(),
			Eth1:       s.eth1HttpAddr,
			GenesisSSZ: s.genesisSSZ,
			Bootnode:   s.bootnodeENR,
		}

		factory, ok := beaconFactory[req.NodeClient]
		if !ok {
			return nil, fmt.Errorf("validator client %s not found", req.NodeClient)
		}

		spec, err := factory(bCfg)
		if err != nil {
			return nil, err
		}
		node, err = deployNode(name, spec)
		if err != nil {
			return nil, err
		}

	} else if typ, ok := req.NodeType.(*proto.NodeDeployRequest_Validator_); ok {
		deploy := typ.Validator

		if deploy.NumValidators == 0 {
			return nil, fmt.Errorf("no number of validators provided")
		}

		// pick a beacon node to connect that is of the same type as the validator
		beacons := s.filterLocked(func(spec *spec.Spec) bool {
			return spec.HasLabel(proto.NodeTypeLabel, proto.NodeType_Beacon.String()) &&
				spec.HasLabel(proto.NodeClientLabel, req.NodeClient.String())
		})
		if len(beacons) == 0 {
			return nil, fmt.Errorf("no beacon node found for client %s", req.NodeClient.String())
		}

		target := beacons[0]
		var accounts []*proto.Account

		if deploy.WithDeposit {
			// send deposits to the genesis contract
			accounts = proto.NewAccounts(int(deploy.NumValidators))

			if err := s.depositHandler.MakeDeposits(accounts); err != nil {
				return nil, err
			}
		} else {
			// deploy from the genesis accounts set
			pickNumAccounts := min(int(deploy.NumValidators), len(s.genesisAccounts))
			if pickNumAccounts == 0 {
				return nil, fmt.Errorf("there are no more genesis accounts to use")
			}
			accounts, s.genesisAccounts = s.genesisAccounts[:pickNumAccounts], s.genesisAccounts[pickNumAccounts:]
		}

		name := fmt.Sprintf("validator-%d-%s", numOfNodes(proto.NodeType_Validator), req.NodeClient.String())

		vCfg := &proto.ValidatorConfig{
			Accounts: accounts,
			Spec:     s.config.Spec.buildConfig(),
			Beacon:   target,
		}

		factory, ok := validatorsFactory[req.NodeClient]
		if !ok {
			return nil, fmt.Errorf("validator client %s not found", req.NodeClient)
		}

		spec, err := factory(vCfg)
		if err != nil {
			return nil, err
		}
		node, err = deployNode(name, spec)
		if err != nil {
			return nil, err
		}
	}

	resp := &proto.NodeDeployResponse{
		Node: node,
	}
	return resp, nil
}

var beaconFactory = map[proto.NodeClient]proto.CreateBeacon2{
	proto.NodeClient_Teku:       components.NewTekuBeacon,
	proto.NodeClient_Prysm:      components.NewPrysmBeacon,
	proto.NodeClient_Lighthouse: components.NewLighthouseBeacon,
}

var validatorsFactory = map[proto.NodeClient]proto.CreateValidator2{
	proto.NodeClient_Teku:       components.NewTekuValidator,
	proto.NodeClient_Prysm:      components.NewPrysmValidator,
	proto.NodeClient_Lighthouse: components.NewLighthouseValidator,
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func (s *Server) NodeList(ctx context.Context, req *proto.NodeListRequest) (*proto.NodeListResponse, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	resp := &proto.NodeListResponse{
		Node: []*proto.Node{},
	}
	for _, n := range s.nodes {
		stub, err := specNodeToNode(n)
		if err != nil {
			return nil, err
		}
		resp.Node = append(resp.Node, stub)
	}
	return resp, nil
}

func (s *Server) NodeStatus(ctx context.Context, req *proto.NodeStatusRequest) (*proto.NodeStatusResponse, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("name is empty")
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	var target spec.Node
	for _, n := range s.nodes {
		if n.Spec().Name == req.Name {
			target = n
		}
	}

	stub, err := specNodeToNode(target)
	if err != nil {
		return nil, err
	}
	resp := &proto.NodeStatusResponse{
		Node: stub,
	}
	return resp, nil
}

func specNodeToNode(n spec.Node) (*proto.Node, error) {
	spec := n.Spec()

	typ, ok := proto.StringToNodeType(spec.Labels[proto.NodeTypeLabel])
	if !ok {
		typ = proto.NodeType_OtherType
	}
	clt, ok := proto.StringToNodeClient(spec.Labels[proto.NodeClientLabel])
	if !ok {
		clt = proto.NodeClient_OtherClient
	}

	resp := &proto.Node{
		Name:   spec.Name,
		Type:   typ,
		Client: clt,
		Labels: spec.Labels,
	}
	return resp, nil
}

type fileLogger struct {
	path string

	lock  sync.Mutex
	files []*os.File
}

func (f *fileLogger) Create(name string) (io.Writer, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	file, err := os.OpenFile(filepath.Join(f.path, name+".log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return nil, err
	}
	if len(f.files) == 0 {
		f.files = []*os.File{}
	}
	f.files = append(f.files, file)
	return file, nil
}

func (f *fileLogger) Close() error {
	for _, file := range f.files {
		// TODO, improve
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}
