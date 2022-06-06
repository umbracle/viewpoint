package server

import (
	"fmt"

	"github.com/umbracle/ethgo/wallet"
	"github.com/umbracle/viewpoint/internal/bls"
)

type ValidatorConfig struct {
	Spec     *Eth2Spec
	Accounts []*Account
	Beacon   *node
}

type BeaconConfig struct {
	Spec     *Eth2Spec
	Eth1     string
	Bootnode string
}

type Account struct {
	Bls   *bls.Key
	Ecdsa *wallet.Key
}

func NewAccounts(num int) []*Account {
	accts := []*Account{}
	for i := 0; i < num; i++ {
		accts = append(accts, NewAccount())
	}
	return accts
}

func NewAccount() *Account {
	key, err := wallet.GenerateKey()
	if err != nil {
		panic(fmt.Errorf("BUG: failed to generate key %v", err))
	}
	account := &Account{
		Bls:   bls.NewRandomKey(),
		Ecdsa: key,
	}
	return account
}

type CreateBeacon2 func(cfg *BeaconConfig) ([]nodeOption, error)

// CreateBeacon is a factory method to create beacon nodes
type CreateBeacon func(cfg *BeaconConfig) (*node, error)

type CreateValidator2 func(cfg *ValidatorConfig) ([]nodeOption, error)

// CreateValidator is a factory method to create validator nodes
type CreateValidator func(cfg *ValidatorConfig) (*node, error)
