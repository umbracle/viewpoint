package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

func TestDepositHandler_Deposit(t *testing.T) {
	d, err := NewDocker()
	assert.NoError(t, err)

	node, err := d.Deploy(NewEth1Server())
	assert.NoError(t, err)
	defer node.Stop()

	handler, err := newDepositHandler(node.GetAddr(NodePortEth1Http))
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
