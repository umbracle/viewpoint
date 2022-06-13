package proto

import (
	"strings"
)

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

	// NodePortP2P is the p2p port for an eth2 node.
	NodePortP2P = "eth2.p2p"

	// NodePortHttp is the http port for an eth2 node.
	NodePortHttp = "eth2.http"

	// NodePortPrysmGrpc is the specific prysm port for its Grpc server
	NodePortPrysmGrpc = "eth2.prysm.grpc"
)
