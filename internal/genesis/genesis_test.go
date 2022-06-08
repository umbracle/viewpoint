package genesis

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

func TestGenesis(t *testing.T) {
	accounts := proto.NewAccounts(10)
	block := &ethgo.Block{}

	state, err := GenerateGenesis(block, 10000, accounts)
	assert.NoError(t, err)

	_, err = state.HashTreeRoot()
	assert.NoError(t, err)
}
