package server

import (
	"context"
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
	"github.com/umbracle/viewpoint/internal/genesis"
	"github.com/umbracle/viewpoint/internal/http"
	"github.com/umbracle/viewpoint/internal/server/proto"
	"google.golang.org/grpc"
)

type Server struct {
	proto.UnimplementedE2EServiceServer
	config *Config
	logger hclog.Logger

	eth1           *node
	depositHandler *depositHandler

	lock       sync.Mutex
	fileLogger *fileLogger
	nodes      []*node
	bootnode   *Bootnode

	// genesis data
	genesisSSZ      []byte
	genesisAccounts []*proto.Account
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	eth1, err := newNode(NewEth1Server()...)
	if err != nil {
		return nil, err
	}
	bootnode, err := NewBootnode()
	if err != nil {
		return nil, err
	}

	// get latest block
	provider, err := jsonrpc.NewClient(eth1.GetAddr(NodePortEth1Http))
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

	depositHandler, err := newDepositHandler(eth1.GetAddr(NodePortEth1Http))
	if err != nil {
		return nil, err
	}

	dataPath := "e2e-" + config.Name

	// create a folder to store data
	if err := os.Mkdir(dataPath, 0755); err != nil {
		return nil, err
	}

	logger.Info("eth1 server deployed", "addr", eth1.GetAddr(NodePortEth1Http))
	logger.Info("deposit contract deployed", "addr", depositHandler.deposit.String())

	config.Spec.DepositContract = depositHandler.deposit.String()

	srv := &Server{
		config: config,
		logger: logger,
		eth1:   eth1,
		nodes: []*node{
			eth1,
			bootnode.node,
		},
		genesisAccounts: accounts,
		fileLogger:      &fileLogger{path: dataPath},
		bootnode:        bootnode,
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

func (s *Server) filterLocked(cond func(opts *nodeOpts) bool) []*node {
	res := []*node{}
	for _, i := range s.nodes {
		if cond(i.opts) {
			res = append(res, i)
		}
	}
	return res
}

func (s *Server) DeployNode(ctx context.Context, req *proto.DeployNodeRequest) (*proto.DeployNodeResponse, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	useBootnode := true

	bCfg := &BeaconConfig{
		Spec:       s.config.Spec,
		Eth1:       s.eth1.GetAddr(NodePortEth1Http),
		GenesisSSZ: s.genesisSSZ,
	}

	if !useBootnode {
		if len(s.nodes) != 0 {
			client := http.NewHttpClient(s.nodes[0].GetAddr(NodePortHttp))
			identity, err := client.NodeIdentity()
			if err != nil {
				return nil, fmt.Errorf("cannto get a bootnode: %v", err)
			}
			bCfg.Bootnode = identity.ENR
		}
	} else {
		bCfg.Bootnode = s.bootnode.Enr
	}

	var beaconFactory CreateBeacon2
	switch req.NodeClient {
	case proto.NodeClient_Teku:
		beaconFactory = NewTekuBeacon
	case proto.NodeClient_Prysm:
		beaconFactory = NewPrysmBeacon
	case proto.NodeClient_Lighthouse:
		beaconFactory = NewLighthouseBeacon
	default:
		return nil, fmt.Errorf("beacon type %s not found", req.NodeClient)
	}

	indx := len(s.nodes)

	// generate a name
	name := fmt.Sprintf("beacon-%s-%d", strings.ToLower(req.NodeClient.String()), indx)

	fLogger, err := s.fileLogger.Create(name)
	if err != nil {
		return nil, err
	}
	opts, err := beaconFactory(bCfg)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"name":     name,
		"type":     "beacon",
		"ensemble": s.config.Name,
	}
	genOpts := []nodeOption{
		WithName(name),
		WithOutput(fLogger),
		WithLabels(labels),
	}
	genOpts = append(genOpts, opts...)

	node, err := newNode(genOpts...)
	if err != nil {
		return nil, err
	}

	s.nodes = append(s.nodes, node)

	s.logger.Info("beacon node started", "client", req.NodeClient, "mount-map", node.mountMap)
	return &proto.DeployNodeResponse{}, nil
}

func min(i, j int) int {
	if i < j {
		return i
	}
	return j
}

func (s *Server) DeployValidator(ctx context.Context, req *proto.DeployValidatorRequest) (*proto.DeployValidatorResponse, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if req.NumValidators == 0 {
		return nil, fmt.Errorf("no number of validators provided")
	}

	beacons := s.filterLocked(func(opts *nodeOpts) bool {
		return opts.NodeType == proto.NodeType_Beacon && opts.NodeClient == req.NodeClient
	})
	if len(beacons) == 0 {
		return nil, fmt.Errorf("no beacon node found for client %s", req.NodeClient.String())
	}

	beacon := beacons[0]

	// deploy from genesis accounts
	pickNumAccounts := min(int(req.NumValidators), len(s.genesisAccounts))
	if pickNumAccounts == 0 {
		return nil, fmt.Errorf("there are no more genesis accounts to use")
	}

	var accounts []*proto.Account
	accounts, s.genesisAccounts = s.genesisAccounts[:pickNumAccounts], s.genesisAccounts[pickNumAccounts:]

	indx := len(s.nodes)

	// generate a name
	name := fmt.Sprintf("validator-%s-%d", strings.ToLower(req.NodeClient.String()), indx)

	fLogger, err := s.fileLogger.Create(name)
	if err != nil {
		return nil, err
	}

	vCfg := &ValidatorConfig{
		Accounts: accounts,
		Spec:     s.config.Spec,
		Beacon:   beacon,
	}

	var validatorFactory CreateValidator2
	switch req.NodeClient {
	case proto.NodeClient_Teku:
		validatorFactory = NewTekuValidator
	case proto.NodeClient_Prysm:
		validatorFactory = NewPrysmValidator
	case proto.NodeClient_Lighthouse:
		validatorFactory = NewLighthouseValidator
	default:
		return nil, fmt.Errorf("validator client %s not found", req.NodeClient)
	}

	opts, err := validatorFactory(vCfg)
	if err != nil {
		return nil, err
	}
	genOpts := []nodeOption{
		WithName(name),
		WithOutput(fLogger),
	}
	genOpts = append(genOpts, opts...)

	node, err := newNode(genOpts...)
	if err != nil {
		return nil, err
	}
	s.nodes = append(s.nodes, node)
	return &proto.DeployValidatorResponse{}, nil
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
