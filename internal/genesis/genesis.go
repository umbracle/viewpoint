package genesis

import (
	"fmt"

	"github.com/umbracle/ethgo"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

var minValidatorBalance = uint64(32000000000)

func GenerateGenesis(eth1Block *ethgo.Block, genesisTime int64, initialValidator []*proto.Account) (*BeaconState, error) {
	if uint64(genesisTime) < eth1Block.Timestamp {
		return nil, fmt.Errorf("low timestamp")
	}
	body := BeaconBlockBody{
		Eth1Data: &Eth1Data{},
	}
	bodyRoot, err := body.HashTreeRoot()
	if err != nil {
		return nil, err
	}

	depositRoot := ethgo.HexToHash("d70a234731285c6804c2a4f56711ddb8c82c99740f207854891028af34e27e5e")

	validators := []*Validator{}
	balances := []uint64{}

	for _, val := range initialValidator {
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

	state := &BeaconState{
		GenesisTime:           uint64(genesisTime), // + 1 minute
		GenesisValidatorsRoot: genesisValidatorRoot,
		Slot:                  0,
		Fork:                  &Fork{},
		LatestBlockHeader: &BeaconBlockHeader{
			BodyRoot: bodyRoot,
		},
		HistoricalRoots: [][32]byte{},
		Eth1Data: &Eth1Data{
			DepositRoot:  depositRoot,
			DepositCount: 0,
			BlockHash:    eth1Block.Hash,
		},
		Eth1DataVotes:               []*Eth1Data{},
		Eth1DepositIndex:            0,
		Validators:                  validators,
		Balances:                    balances,
		Slashings:                   slashings,
		PreviousEpochAttestations:   []*PendingAttestation{},
		CurrentEpochAttestations:    []*PendingAttestation{},
		PreviousJustifiedCheckpoint: &Checkpoint{},
		CurrentJustifiedCheckpoint:  &Checkpoint{},
		FinalizedCheckpoint:         &Checkpoint{},
	}
	return state, nil
}
