package genesis

import (
	"fmt"

	ssz "github.com/ferranbt/fastssz"
	"github.com/umbracle/ethgo"
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

func GenerateGenesis(input *Input) (ssz.Marshaler, error) {
	if uint64(input.GenesisTime) < input.Eth1Block.Timestamp {
		return nil, fmt.Errorf("low timestamp")
	}

	depositRoot := ethgo.HexToHash("d70a234731285c6804c2a4f56711ddb8c82c99740f207854891028af34e27e5e")

	validators := []*Validator{}
	balances := []uint64{}

	for _, val := range input.InitialValidator {
		pubKey := val.Bls.PubKey()

		validators = append(validators, &Validator{
			Pubkey:                     pubKey[:],
			WithdrawalCredentials:      make([]byte, 32),
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

	fork := &Fork{
		CurrentVersion: input.ForkVersion,
	}

	var state ssz.Marshaler
	if input.Fork == proto.Fork_Phase0 {
		body := BeaconBlockBodyPhase0{
			Eth1Data: &Eth1Data{},
		}
		bodyRoot, err := body.HashTreeRoot()
		if err != nil {
			return nil, err
		}

		state = &BeaconStatePhase0{
			GenesisTime:           uint64(input.GenesisTime), // + 1 minute
			GenesisValidatorsRoot: genesisValidatorRoot,
			Fork:                  fork,
			LatestBlockHeader: &BeaconBlockHeader{
				BodyRoot: bodyRoot,
			},
			Eth1Data: &Eth1Data{
				DepositRoot: depositRoot,
				BlockHash:   input.Eth1Block.Hash,
			},
			Validators: validators,
			Balances:   balances,
			Slashings:  slashings,
		}
	} else if input.Fork == proto.Fork_Altair {
		body := BeaconBlockBodyAltair{
			Eth1Data:      &Eth1Data{},
			SyncAggregate: &SyncAggregate{},
		}
		bodyRoot, err := body.HashTreeRoot()
		if err != nil {
			return nil, err
		}

		state = &BeaconStateAltair{
			GenesisTime:           uint64(input.GenesisTime), // + 1 minute
			GenesisValidatorsRoot: genesisValidatorRoot,
			Fork:                  fork,
			LatestBlockHeader: &BeaconBlockHeader{
				BodyRoot: bodyRoot,
			},
			Eth1Data: &Eth1Data{
				DepositRoot: depositRoot,
				BlockHash:   input.Eth1Block.Hash,
			},
			Validators: validators,
			Balances:   balances,
			Slashings:  slashings,
		}
	}

	return state, nil
}
