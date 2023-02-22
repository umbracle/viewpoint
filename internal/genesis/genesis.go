package genesis

import (
	"encoding/hex"
	"fmt"

	ssz "github.com/ferranbt/fastssz"
	"github.com/umbracle/ethgo"
	consensus "github.com/umbracle/go-eth-consensus"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

var minValidatorBalance = uint64(32000000000)

type Input struct {
	Eth1Block        *ethgo.Block
	GenesisTime      int64
	InitialValidator []*proto.Account
	Fork             proto.Fork
	ForkVersion      [4]byte
}

var emptyDepositRoot = [32]byte{}

func init() {
	b, err := hex.DecodeString("d70a234731285c6804c2a4f56711ddb8c82c99740f207854891028af34e27e5e")
	if err != nil {
		panic(fmt.Sprintf("BUG: failed to parse empty deposit"))
	}
	copy(emptyDepositRoot[:], b)
}

func GenerateGenesis(input *Input) (ssz.Marshaler, error) {
	if uint64(input.GenesisTime) < input.Eth1Block.Timestamp {
		return nil, fmt.Errorf("low timestamp")
	}

	validators := []*consensus.Validator{}
	balances := []uint64{}

	for _, val := range input.InitialValidator {
		pubKey := val.Bls.PubKey()

		validators = append(validators, &consensus.Validator{
			Pubkey:                     pubKey,
			ActivationEligibilityEpoch: 0,
			ActivationEpoch:            0,
			EffectiveBalance:           minValidatorBalance,
			WithdrawableEpoch:          18446744073709551615,
			ExitEpoch:                  18446744073709551615,
		})

		balances = append(balances, minValidatorBalance)
	}

	validatorSet := &ValidatorSet{
		Set: validators,
	}
	genesisValidatorRoot, err := validatorSet.HashTreeRoot()
	if err != nil {
		return nil, err
	}

	slashings := []uint64{}
	for i := 0; i < 8192; i++ {
		slashings = append(slashings, 0)
	}

	fork := &consensus.Fork{
		CurrentVersion: input.ForkVersion,
	}

	var state ssz.Marshaler
	if input.Fork == proto.Fork_Phase0 {
		body := consensus.BeaconBlockBodyPhase0{
			Eth1Data: &consensus.Eth1Data{},
		}
		bodyRoot, err := body.HashTreeRoot()
		if err != nil {
			return nil, err
		}

		state = &consensus.BeaconStatePhase0{
			GenesisTime:           uint64(input.GenesisTime), // + 1 minute
			GenesisValidatorsRoot: genesisValidatorRoot,
			Fork:                  fork,
			LatestBlockHeader: &consensus.BeaconBlockHeader{
				BodyRoot: bodyRoot,
			},
			Eth1Data: &consensus.Eth1Data{
				DepositRoot: emptyDepositRoot,
				BlockHash:   input.Eth1Block.Hash,
			},
			Validators: validators,
			Balances:   balances,
			Slashings:  slashings,
		}
	} else if input.Fork == proto.Fork_Altair {
		body := consensus.BeaconBlockBodyAltair{
			Eth1Data:      &consensus.Eth1Data{},
			SyncAggregate: &consensus.SyncAggregate{},
		}
		bodyRoot, err := body.HashTreeRoot()
		if err != nil {
			return nil, err
		}

		state = &consensus.BeaconStateAltair{
			GenesisTime:           uint64(input.GenesisTime), // + 1 minute
			GenesisValidatorsRoot: genesisValidatorRoot,
			Fork:                  fork,
			LatestBlockHeader: &consensus.BeaconBlockHeader{
				BodyRoot: bodyRoot,
			},
			Eth1Data: &consensus.Eth1Data{
				DepositRoot: emptyDepositRoot,
				BlockHash:   input.Eth1Block.Hash,
			},
			Validators: validators,
			Balances:   balances,
			Slashings:  slashings,
		}
	}

	return state, nil
}

type ValidatorSet struct {
	Set []*consensus.Validator `ssz-max:"1099511627776"`
}

// HashTreeRootWith ssz hashes the ValidatorSet object with a hasher
func (v *ValidatorSet) HashTreeRoot() (root [32]byte, err error) {
	hh := ssz.NewHasher()

	indx := hh.Index()

	// Field (0) 'Set'
	{
		subIndx := hh.Index()
		num := uint64(len(v.Set))
		if num > 1099511627776 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for _, elem := range v.Set {
			if err = elem.HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 1099511627776)
	}

	hh.Merkleize(indx)

	root, err = hh.HashRoot()
	return
}
