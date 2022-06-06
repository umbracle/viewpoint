package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
)

func TestDepositHandler_Deposit(t *testing.T) {
	node, err := newNode(NewEth1Server()...)
	assert.NoError(t, err)

	handler, err := newDepositHandler(node.GetAddr(NodePortEth1Http))
	assert.NoError(t, err)

	code, err := handler.Provider().Eth().GetCode(handler.deposit, ethgo.Latest)
	assert.NoError(t, err)
	assert.NotEqual(t, code, "0x")

	account := NewAccount()

	err = handler.MakeDeposit(account)
	assert.NoError(t, err)

	contract := handler.GetDepositContract()
	count, err := contract.GetDepositCount(ethgo.Latest)
	assert.NoError(t, err)
	assert.Equal(t, int(count[0]), 1)
}
