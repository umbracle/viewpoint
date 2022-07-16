package server

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
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

	tranches map[uint64]*Tranche

	// genesis data
	genesisSSZ []byte
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	// for simplicity we force that there is a perfect division between
	// the initial validators and the tranches
	if config.Spec.GenesisValidatorCount%int(config.NumTranches) != 0 {
		return nil, fmt.Errorf("genesis validator count not multiple of the tranches, got %d and %d", config.Spec.GenesisValidatorCount, config.NumTranches)
	}

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
		fileLogger:     &fileLogger{path: dataPath},
		bootnodeENR:    bootnodeSpec.Enr,
		depositHandler: depositHandler,
		tranches:       map[uint64]*Tranche{},
	}

	// create the tranches and initial accounts
	numAccountsPerTranche := config.Spec.GenesisValidatorCount / int(config.NumTranches)

	initialAccounts := []*proto.Account{}
	for i := 0; i < int(config.NumTranches); i++ {
		tranche, err := srv.createTranche(numAccountsPerTranche, false)
		if err != nil {
			return nil, err
		}
		initialAccounts = append(initialAccounts, tranche.Accounts...)
	}

	state, err := genesis.GenerateGenesis(block, int64(config.Spec.MinGenesisTime), initialAccounts)
	if err != nil {
		return nil, err
	}
	srv.genesisSSZ, err = state.MarshalSSZ()
	if err != nil {
		return nil, err
	}
	if _, err := srv.writeFile("spec.yaml", config.Spec.buildConfig()); err != nil {
		return nil, err
	}
	if _, err := srv.writeFile("genesis.ssz", srv.genesisSSZ); err != nil {
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

func (s *Server) writeFile(path string, content []byte) (string, error) {
	pwdPath, err := os.Getwd()
	if err != nil {
		return "", err
	}

	fullPath := filepath.Join(pwdPath, "e2e-"+s.config.Name, path)

	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", err
	}
	return fullPath, nil
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

type Tranche struct {
	Accounts  []*proto.Account
	Filepath  string
	Validator string
}

func (t *Tranche) ToProto() (*proto.TrancheStub, error) {
	res := &proto.TrancheStub{
		Name: t.Validator,
		Path: t.Filepath,
	}
	for _, acct := range t.Accounts {
		stub, err := acct.ToStub()
		if err != nil {
			return nil, err
		}
		res.Accounts = append(res.Accounts, stub)
	}
	return res, nil
}

func (t *Tranche) IsConsumed() bool {
	return t.Validator != ""
}

// createTranche creates a new tranche object including the deposits
func (s *Server) createTranche(numValidators int, deposit bool) (*Tranche, error) {
	accounts := proto.NewAccounts(numValidators)

	if deposit {
		if err := s.depositHandler.MakeDeposits(accounts); err != nil {
			return nil, err
		}
	}

	// create a tranche file on the datadir
	privKeys := []string{}
	for _, acct := range accounts {
		priv, err := acct.Bls.Prv.Marshal()
		if err != nil {
			return nil, err
		}
		privKeys = append(privKeys, hex.EncodeToString(priv[:]))
	}

	numTranches := len(s.tranches)
	tranchPath, err := s.writeFile(fmt.Sprintf("tranche_%d.txt", numTranches), []byte(strings.Join(privKeys, "\n")))
	if err != nil {
		return nil, err
	}

	tranche := &Tranche{
		Accounts: accounts,
		Filepath: tranchPath,
	}
	s.tranches[uint64(numTranches)] = tranche

	s.logger.Info("tranche created", "index", uint64(numTranches), "num-accounts", len(accounts), "path", tranchPath)
	return tranche, nil
}

func (s *Server) DepositList(ctx context.Context, req *proto.DepositListRequest) (*proto.DepositListResponse, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	res := []*proto.TrancheStub{}
	for index, tranche := range s.tranches {
		stub, err := tranche.ToProto()
		if err != nil {
			return nil, err
		}
		stub.Index = index
		res = append(res, stub)
	}

	resp := &proto.DepositListResponse{
		Tranches: res,
	}
	return resp, nil
}

func (s *Server) DepositCreate(ctx context.Context, req *proto.DepositCreateRequest) (*proto.DepositCreateResponse, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	tranche, err := s.createTranche(int(req.NumValidators), true)
	if err != nil {
		return nil, err
	}

	stub, err := tranche.ToProto()
	if err != nil {
		return nil, err
	}
	resp := &proto.DepositCreateResponse{
		Tranche: stub,
	}
	return resp, nil
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

	createdNodes := []*proto.Node{}

	deployNode := func(name string, spec *spec.Spec) (spec.Node, error) {
		fLogger, err := s.fileLogger.Create(name)
		if err != nil {
			return nil, err
		}
		spec.WithName(name).
			WithOutput(fLogger)

		if req.Repo != "" {
			spec = spec.WithContainer(req.Repo)
		}
		if req.Tag != "" {
			spec = spec.WithTag(req.Tag)
		}

		node, err := s.docker.Deploy(spec)
		if err != nil {
			return nil, err
		}
		s.nodes = append(s.nodes, node)

		stub, err := specNodeToNode(node)
		if err != nil {
			return nil, err
		}
		createdNodes = append(createdNodes, stub)
		return node, nil
	}

	deployBeacon := func() (spec.Node, error) {
		name := fmt.Sprintf("beacon-%d-%s", numOfNodes(proto.NodeType_Beacon), strings.ToLower(req.NodeClient.String()))
		s.logger.Info("deploy beacon node", "name", name)

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
		node, err := deployNode(name, spec)
		if err != nil {
			return nil, err
		}
		return node, nil
	}

	deployValidator := func(deploy *proto.NodeDeployRequest_Validator, target spec.Node) (spec.Node, error) {
		if target == nil {
			// pick a beacon node to connect that is of the same type as the validator
			beacons := s.filterLocked(func(spec *spec.Spec) bool {
				return spec.HasLabel(proto.NodeTypeLabel, proto.NodeType_Beacon.String()) &&
					spec.HasLabel(proto.NodeClientLabel, req.NodeClient.String())
			})
			if len(beacons) == 0 {
				return nil, fmt.Errorf("no beacon node found for client %s", req.NodeClient.String())
			}
			target = beacons[0]
		}

		var tranche *Tranche
		if deploy.NumValidators == 0 {
			// a tranch is selected from the list
			var ok bool
			tranche, ok = s.tranches[deploy.NumTranch]
			if !ok {
				return nil, fmt.Errorf("tranche number '%d' does not exists", deploy.NumTranch)
			}
			if tranche.IsConsumed() {
				return nil, fmt.Errorf("tranche '%d' has already been used", deploy.NumTranch)
			}
		} else {
			// create a new tranch (with deposit)
			var err error
			if tranche, err = s.createTranche(int(deploy.NumValidators), true); err != nil {
				return nil, err
			}
		}

		name := fmt.Sprintf("validator-%d-%s", numOfNodes(proto.NodeType_Validator), strings.ToLower(req.NodeClient.String()))
		s.logger.Info("deploy validator node", "name", name)

		vCfg := &proto.ValidatorConfig{
			Accounts: tranche.Accounts,
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
		node, err := deployNode(name, spec)
		if err != nil {
			return nil, err
		}

		// consume the tranche
		tranche.Validator = name
		return node, nil
	}

	beaconReq, ok := req.NodeType.(*proto.NodeDeployRequest_Beacon_)
	if !ok {
		// we still have to deploy beacon nodes if requested by a validator
		if valReq := req.NodeType.(*proto.NodeDeployRequest_Validator_); valReq.Validator.WithBeacon {
			beaconReq = &proto.NodeDeployRequest_Beacon_{
				Beacon: &proto.NodeDeployRequest_Beacon{
					Count: valReq.Validator.BeaconCount,
				},
			}
		}
	}

	var beacons []spec.Node
	if beaconReq != nil {
		// deploy beacon nodes
		for i := 0; i < int(beaconReq.Beacon.Count); i++ {
			beacon, err := deployBeacon()
			if err != nil {
				return nil, err
			}
			beacons = append(beacons, beacon)
		}
	}

	// deploy validator if requested
	if valReq, ok := req.NodeType.(*proto.NodeDeployRequest_Validator_); ok {
		// if also deployed a beacon, use one of those as a target
		var target spec.Node
		if len(beacons) != 0 {
			target = beacons[0]
		}
		// deploy validator
		if _, err := deployValidator(valReq.Validator, target); err != nil {
			return nil, err
		}
	}

	resp := &proto.NodeDeployResponse{
		Nodes: createdNodes,
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
