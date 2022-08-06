package proto

import (
	"fmt"

	"github.com/umbracle/ethgo/wallet"
	"github.com/umbracle/go-eth-consensus/bls"
)

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
