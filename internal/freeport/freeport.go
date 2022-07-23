package freeport

import (
	"fmt"
	"net"
	"sync"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

var (
	mu sync.Mutex
)

// port ranges for each node value.
var ports = map[proto.NodePort]uint64{
	proto.NodePortEth1Http:    8000,
	proto.NodePortEth1P2P:     9000,
	proto.NodePortEth1AuthRPC: 6000,
	proto.NodePortP2P:         5000,
	proto.NodePortHttp:        7000,
	proto.NodePortPrysmGrpc:   4000,
	proto.NodePortBootnode:    3000,
}

func Take(n proto.NodePort) uint64 {
	mu.Lock()
	defer mu.Unlock()

	port, ok := ports[n]
	if !ok {
		panic(fmt.Sprintf("name not known: %s", n))
	}

	for {
		if !isPortInUse(port) {
			break
		}
	}

	ports[n] = port + 1
	return port
}

func isPortInUse(port uint64) bool {
	ln, err := net.ListenTCP("tcp", tcpAddr("127.0.0.1", port))
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

func tcpAddr(ip string, port uint64) *net.TCPAddr {
	return &net.TCPAddr{IP: net.ParseIP(ip), Port: int(port)}
}
