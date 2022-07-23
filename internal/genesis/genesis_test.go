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

	input := &Input{
		Eth1Block:        block,
		GenesisTime:      10000,
		InitialValidator: accounts,
	}
	state, err := GenerateGenesis(input)
	assert.NoError(t, err)

	_, err = state.MarshalSSZ()
	assert.NoError(t, err)
}
