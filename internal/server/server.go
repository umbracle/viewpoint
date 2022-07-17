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
	logDir      *logDir
	nodes       []spec.Node
	bootnodeENR string

	tranches map[uint64]*Tranche

	// genesis data
	genesisSSZ []byte
}

func NewServer(logger hclog.Logger, config *Config) (*Server, error) {
	// for simplicity we force that there is a perfect division between
	// the initial validators and the tranches
	if config.NumGenesisValidators%config.NumTranches != 0 {
		return nil, fmt.Errorf("genesis validator count not multiple of the tranches, got %d and %d", config.NumGenesisValidators, config.NumTranches)
	}

	docker, err := docker.NewDocker()
	if err != nil {
		return nil, err
	}

	logDir, err := newLogDir("e2e-" + config.Name)
	if err != nil {
		return nil, err
	}

	srv := &Server{
		config:   config,
		logger:   logger,
		docker:   docker,
		nodes:    []spec.Node{},
		logDir:   logDir,
		tranches: map[uint64]*Tranche{},
	}

	// deploy bootnode
	bootnode := components.NewBootnode()
	if _, err := srv.deployNode(bootnode.Spec.WithName("bootnode")); err != nil {
		return nil, err
	}
	srv.bootnodeENR = bootnode.Enr

	// deploy eth1 node
	eth1, err := srv.deployNode(components.NewEth1Server().WithName("eth1"))
	if err != nil {
		return nil, err
	}
	srv.eth1HttpAddr = eth1.GetAddr(proto.NodePortEth1Http)
	logger.Info("eth1 server deployed", "addr", srv.eth1HttpAddr)

	// deploy depositHandler
	if srv.depositHandler, err = newDepositHandler(srv.eth1HttpAddr); err != nil {
		return nil, err
	}
	logger.Info("deposit contract deployed", "addr", srv.depositHandler.deposit.String())
	config.Spec.DepositContract = srv.depositHandler.deposit.String()

	// setup the genesis.ssz file
	if err := srv.setupGenesis(); err != nil {
		return nil, fmt.Errorf("failed to create genesis.szz file: %v", err)
	}

	// start the grpc server
	if err := srv.setupGrpcServer(); err != nil {
		return nil, fmt.Errorf("failed to start grpc server: %v", err)
	}

	return srv, nil
}

func (s *Server) setupGenesis() error {
	// create the tranches and initial accounts
	numAccountsPerTranche := s.config.NumGenesisValidators / s.config.NumTranches

	initialAccounts := []*proto.Account{}
	for i := 0; i < int(s.config.NumTranches); i++ {
		tranche, err := s.createTranche(int(numAccountsPerTranche), false)
		if err != nil {
			return err
		}
		initialAccounts = append(initialAccounts, tranche.Accounts...)
	}

	// get the latest block from the eth1 chain to create the genesis
	provider, err := jsonrpc.NewClient(s.eth1HttpAddr)
	if err != nil {
		return err
	}
	block, err := provider.Eth().GetBlockByNumber(ethgo.Latest, true)
	if err != nil {
		return err
	}

	state, err := genesis.GenerateGenesis(block, int64(s.config.Spec.MinGenesisTime), initialAccounts)
	if err != nil {
		return err
	}
	s.genesisSSZ, err = state.MarshalSSZ()
	if err != nil {
		return err
	}
	if _, err := s.logDir.writeFile("spec.yaml", s.config.Spec.buildConfig()); err != nil {
		return err
	}
	if _, err := s.logDir.writeFile("genesis.ssz", s.genesisSSZ); err != nil {
		return err
	}
	return nil
}

func (s *Server) setupGrpcServer() error {
	grpcServer := grpc.NewServer()
	proto.RegisterE2EServiceServer(grpcServer, s)

	grpcAddr := "localhost:5555"
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		return err
	}

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			s.logger.Error("failed to serve grpc server", "err", err)
		}
	}()
	s.logger.Info("GRPC Server started", "addr", grpcAddr)
	return nil
}

func (s *Server) deployNode(spec *spec.Spec) (spec.Node, error) {
	fLogger, err := s.logDir.CreateLogFile(spec.Name)
	if err != nil {
		return nil, err
	}
	spec.WithOutput(fLogger).
		WithLabel("viewpoint", "true").
		WithLabel("env", s.config.Name)

	node, err := s.docker.Deploy(spec)
	if err != nil {
		return nil, err
	}
	s.nodes = append(s.nodes, node)
	return node, nil
}

func (s *Server) Stop() {
	// stop all servers
	for _, node := range s.nodes {
		if err := node.Stop(); err != nil {
			s.logger.Error("failed to stop node", "id", "x", "err", err)
		}
	}
	if err := s.logDir.Close(); err != nil {
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
	tranchPath, err := s.logDir.writeFile(fmt.Sprintf("tranche_%d.txt", numTranches), []byte(strings.Join(privKeys, "\n")))
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
		spec.WithName(name)
		if req.Repo != "" {
			spec = spec.WithContainer(req.Repo)
		}
		if req.Tag != "" {
			spec = spec.WithTag(req.Tag)
		}

		node, err := s.deployNode(spec)
		if err != nil {
			return nil, err
		}
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

type logDir struct {
	path string

	lock     sync.Mutex
	logFiles []*os.File
}

func newLogDir(path string) (*logDir, error) {
	pwdPath, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	path = filepath.Join(pwdPath, path)
	if err := os.Mkdir(path, 0755); err != nil {
		// it fails if path already exists
		return nil, err
	}

	logDir := &logDir{
		path: path,
	}
	return logDir, nil
}

func (l *logDir) writeFile(path string, content []byte) (string, error) {
	fullPath := filepath.Join(l.path, path)

	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0700); err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(fullPath, []byte(content), 0644); err != nil {
		return "", err
	}
	return fullPath, nil
}

func (l *logDir) CreateLogFile(name string) (io.Writer, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	file, err := os.OpenFile(filepath.Join(l.path, name+".log"), os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return nil, err
	}
	if len(l.logFiles) == 0 {
		l.logFiles = []*os.File{}
	}
	l.logFiles = append(l.logFiles, file)
	return file, nil
}

func (l *logDir) Close() error {
	for _, file := range l.logFiles {
		// TODO, improve
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}
