package server

import (
	"fmt"
	"net/http"
	"time"

	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
	"github.com/umbracle/ethgo/jsonrpc"
	"github.com/umbracle/viewpoint/internal/deposit"
)

// depositHandler is an eth1x testutil server using go-ethereum
type depositHandler struct {
	deposit ethgo.Address
	client  *jsonrpc.Client
}

func newDepositHandler(eth1Addr string) (*depositHandler, error) {
	provider, err := jsonrpc.NewClient(eth1Addr)
	if err != nil {
		return nil, err
	}
	handler := &depositHandler{
		client: provider,
	}
	if err := handler.deployDeposit(); err != nil {
		return nil, err
	}
	return handler, nil
}

const (
	defaultGasPrice = 1879048192 // 0x70000000
	defaultGasLimit = 5242880    // 0x500000
)

func (e *depositHandler) fund(addr ethgo.Address) error {
	nonce, err := e.client.Eth().GetNonce(addr, ethgo.Latest)
	if err != nil {
		return err
	}

	txn := &ethgo.Transaction{
		From:     e.Owner(),
		To:       &addr,
		Nonce:    nonce,
		GasPrice: defaultGasPrice,
		Gas:      defaultGasLimit,
		// fund the account with enoung balance to validate and send the transaction
		Value: ethgo.Ether(deposit.MinGweiAmount + 1),
	}
	hash, err := e.client.Eth().SendTransaction(txn)
	if err != nil {
		return err
	}
	if _, err := e.waitForReceipt(hash); err != nil {
		return err
	}
	return nil
}

func (e *depositHandler) waitForReceipt(hash ethgo.Hash) (*ethgo.Receipt, error) {
	var count uint64
	for {
		receipt, err := e.client.Eth().GetTransactionReceipt(hash)
		if err != nil {
			if err.Error() != "not found" {
				return nil, err
			}
		}
		if receipt != nil {
			return receipt, nil
		}
		if count > 120 {
			break
		}
		time.Sleep(1 * time.Second)
		count++
	}
	return nil, fmt.Errorf("timeout")
}

// Provider returns the jsonrpc provider
func (e *depositHandler) Provider() *jsonrpc.Client {
	return e.client
}

// Owner returns the account with balance on go-ethereum
func (e *depositHandler) Owner() ethgo.Address {
	owner, _ := e.Provider().Eth().Accounts()
	return owner[0]
}

// DeployDeposit deploys the eth2.0 deposit contract
func (e *depositHandler) deployDeposit() error {
	provider := e.Provider()

	owner, err := provider.Eth().Accounts()
	if err != nil {
		return err
	}

	deployTxn := &ethgo.Transaction{
		Input: deposit.DepositBin(),
		From:  owner[0],
	}
	hash, err := provider.Eth().SendTransaction(deployTxn)
	if err != nil {
		return err
	}
	receipt, err := e.waitForReceipt(hash)
	if err != nil {
		return err
	}
	e.deposit = receipt.ContractAddress
	return nil
}

func (e *depositHandler) Deposit() ethgo.Address {
	return e.deposit
}

func (e *depositHandler) GetDepositContract() *deposit.Deposit {
	return deposit.NewDeposit(e.deposit, contract.WithJsonRPC(e.Provider().Eth()))
}

// MakeDeposits deposits the minimum required value to become a validator to multiple accounts
func (e *depositHandler) MakeDeposits(accounts []*Account) error {
	errCh := make(chan error, len(accounts))
	for _, acct := range accounts {
		go func(acct *Account) {
			errCh <- e.MakeDeposit(acct)
		}(acct)
	}

	for i := 0; i < len(accounts); i++ {
		if err := <-errCh; err != nil {
			return err
		}
	}
	return nil
}

// MakeDeposit deposits the minimum required value to become a validator
func (e *depositHandler) MakeDeposit(account *Account) error {
	depositAmount := deposit.MinGweiAmount

	// fund the owner address
	if err := e.fund(account.Ecdsa.Address()); err != nil {
		return err
	}

	data, err := deposit.Input(account.Bls, nil, ethgo.Gwei(depositAmount).Uint64())
	if err != nil {
		return err
	}

	depositC := deposit.NewDeposit(e.deposit, contract.WithSender(account.Ecdsa), contract.WithJsonRPC(e.Provider().Eth()))

	txn, err := depositC.Deposit(data.Pubkey, data.WithdrawalCredentials, data.Signature, data.Root)
	if err != nil {
		return err
	}
	txn.WithOpts(&contract.TxnOpts{Value: ethgo.Ether(depositAmount)})

	if err := txn.Do(); err != nil {
		return err
	}
	if _, err := txn.Wait(); err != nil {
		return err
	}
	return nil
}

func testHTTPEndpoint(endpoint string) error {
	resp, err := http.Post(endpoint, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
