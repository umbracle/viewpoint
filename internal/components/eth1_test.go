package components

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
	"github.com/umbracle/viewpoint/internal/docker"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

func TestEth1_Cluster(t *testing.T) {
	d, err := docker.NewDocker()
	assert.NoError(t, err)

	bootnodev4 := NewBootnodeV4()
	_, err = d.Deploy(bootnodev4.Spec)
	assert.NoError(t, err)

	genesis, key, err := NewDevGenesis()
	assert.NoError(t, err)

	genesisRaw, err := genesis.Build()
	assert.NoError(t, err)

	// start the validators. It only starts one for now.
	config := &proto.ExecutionConfig{
		Bootnode: bootnodev4.Enode,
		Genesis:  string(genesisRaw),
		Key:      key,
	}
	node, err := d.Deploy(NewEth1Server(config))
	assert.NoError(t, err)
	defer node.Stop()

	// start n non-validator nodes
	nonValidators := 3
	for i := 0; i < nonValidators; i++ {
		config := &proto.ExecutionConfig{
			Bootnode: bootnodev4.Enode,
			Genesis:  string(genesisRaw),
		}
		node, err := d.Deploy(NewEth1Server(config))
		assert.NoError(t, err)
		defer node.Stop()
	}

	// check the balance of the premined account
	fmt.Println("-jsonrpc eth single test-", node.GetAddr(proto.NodePortEth1Http))
	client, err := jsonrpc.NewClient(node.GetAddr(proto.NodePortEth1Http))
	assert.NoError(t, err)

	balance, err := client.Eth().GetBalance(key.Address(), ethgo.Latest)
	assert.NoError(t, err)
	assert.Equal(t, balance.String(), genesis.Allocs[key.Address()])
}

func TestEth1_BuildGenesis(t *testing.T) {
	e := &Eth1Genesis{
		Allocs: map[ethgo.Address]string{
			{}:    "10000000000",
			{0x1}: "10000000000",
		},
		Validators: []ethgo.Address{
			{0x1},
			{0x2},
			{0x3},
		},
	}
	_, err := e.Build()
	assert.NoError(t, err)
}
