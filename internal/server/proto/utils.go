package proto

import (
	"encoding/hex"
	"strings"

	"github.com/umbracle/ethgo/wallet"
	"github.com/umbracle/viewpoint/internal/spec"
)

func StringToNodeType(str string) (NodeType, bool) {
	found, ok := NodeType_value[strings.Title(str)]
	if !ok {
		return 0, false
	}
	return NodeType(found), true
}

func StringToNodeClient(str string) (NodeClient, bool) {
	found, ok := NodeClient_value[strings.Title(str)]
	if !ok {
		return 0, false
	}
	return NodeClient(found), true
}

type NodePort string

const (
	// NodePortEth1Http is the http port for the eth1 node.
	NodePortEth1Http = "eth1.http"

	// NodePortEth1P2P is the p2p port for an eth1 node.
	NodePortEth1P2P = "eth1.p2p"

	// NodePortEth1AuthRPC is the p2p port for the eth1 authrpc endpoint.
	NodePortEth1AuthRPC = "eth1.authrpc"

	// NodePortP2P is the p2p port for an eth2 node.
	NodePortP2P = "eth2.p2p"

	// NodePortHttp is the http port for an eth2 node.
	NodePortHttp = "eth2.http"

	// NodePortPrysmGrpc is the specific prysm port for its Grpc server
	NodePortPrysmGrpc = "eth2.prysm.grpc"

	// NodePortBootnode is the port for the bootnode
	NodePortBootnode = "eth.bootnode"
)

func (n NodePort) IsTCP() bool {
	return n != NodePortBootnode
}

const (
	NodeClientLabel = "NodeClient"
	NodeTypeLabel   = "NodeType"
)

type ValidatorConfig struct {
	Spec     []byte
	Accounts []*Account
	Beacon   spec.Node
}

type BeaconConfig struct {
	Spec       []byte
	Eth1       string
	Bootnode   string
	GenesisSSZ []byte
}

type ExecutionConfig struct {
	Bootnode string
	Genesis  string
	Key      *wallet.Key
}

type CreateBeacon2 func(cfg *BeaconConfig) (*spec.Spec, error)

type CreateValidator2 func(cfg *ValidatorConfig) (*spec.Spec, error)

type IsNodeDeployRequest_NodeType interface {
	isNodeDeployRequest_NodeType
}

func (a *Account) ToStub() (*AccountStub, error) {
	priv, err := a.Bls.Prv.Marshal()
	if err != nil {
		return nil, err
	}

	pubKey := a.Bls.Pub.Serialize()
	stub := &AccountStub{
		PrivKey: hex.EncodeToString(priv),
		PubKey:  hex.EncodeToString(pubKey[:]),
	}
	return stub, nil
}
