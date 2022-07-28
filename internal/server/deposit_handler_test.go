package server

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/jsonrpc"
	"github.com/umbracle/viewpoint/internal/components"
	"github.com/umbracle/viewpoint/internal/docker"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

func TestDepositHandler_Deposit(t *testing.T) {
	d, err := docker.NewDocker()
	assert.NoError(t, err)

	genesis, key, err := components.NewDevGenesis()
	assert.NoError(t, err)

	genesisRaw, err := genesis.Build()
	assert.NoError(t, err)

	config := &proto.ExecutionConfig{
		Genesis: genesisRaw,
		Key:     key,
	}
	node, err := d.Deploy(components.NewEth1Server(config))
	assert.NoError(t, err)
	defer node.Stop()

	{
		// check the balance of the premined account
		fmt.Println("-jsonrpc-", node.GetAddr(proto.NodePortEth1Http))
		client, err := jsonrpc.NewClient(node.GetAddr(proto.NodePortEth1Http))
		assert.NoError(t, err)

		balance, err := client.Eth().GetBalance(key.Address(), ethgo.Latest)
		assert.NoError(t, err)
		assert.Equal(t, balance.String(), genesis.Allocs[key.Address()])
	}

	handler, err := newDepositHandler(node.GetAddr(proto.NodePortEth1Http), key)
	assert.NoError(t, err)

	code, err := handler.Provider().Eth().GetCode(handler.deposit, ethgo.Latest)
	assert.NoError(t, err)
	assert.NotEqual(t, code, "0x")

	round, numAccounts := 3, 5
	for i := 0; i < round; i++ {
		accounts := make([]*proto.Account, numAccounts)
		for j := 0; j < numAccounts; j++ {
			accounts[j] = proto.NewAccount()
		}
		err = handler.MakeDeposits(accounts)
		assert.NoError(t, err)
	}

	count, err := handler.GetDepositCount()
	assert.NoError(t, err)
	assert.Equal(t, count, uint32(round*numAccounts))
}
