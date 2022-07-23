package server

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
	"github.com/umbracle/ethgo/contract"
	"github.com/umbracle/ethgo/jsonrpc"
	"github.com/umbracle/ethgo/wallet"
	"github.com/umbracle/viewpoint/internal/deposit"
	"github.com/umbracle/viewpoint/internal/server/proto"
)

// depositHandler is an eth1x testutil server using go-ethereum
type depositHandler struct {
	deposit ethgo.Address
	client  *jsonrpc.Client
	key     *wallet.Key
	nonce   int64
}

func newDepositHandler(eth1Addr string, key *wallet.Key) (*depositHandler, error) {
	provider, err := jsonrpc.NewClient(eth1Addr)
	if err != nil {
		return nil, err
	}
	handler := &depositHandler{
		client: provider,
		key:    key,
		nonce:  -1,
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
	_, err := e.sendTransaction(&ethgo.Transaction{
		To: &addr,
		// fund the account with enoung balance to validate and send the transaction
		Value: ethgo.Ether(deposit.MinGweiAmount + 1),
	})
	return err
}

func (e *depositHandler) getNextNonce() uint64 {
	return uint64(atomic.AddInt64(&e.nonce, 1))
}

func (e *depositHandler) sendTransaction(txn *ethgo.Transaction) (*ethgo.Receipt, error) {
	if txn.GasPrice == 0 {
		txn.GasPrice = defaultGasPrice
	}
	if txn.Gas == 0 {
		txn.Gas = defaultGasLimit
	}
	if txn.Nonce == 0 {
		txn.Nonce = e.getNextNonce()
	}

	signer := wallet.NewEIP155Signer(1337)
	signedTxn, err := signer.SignTx(txn, e.key)
	if err != nil {
		return nil, err
	}

	txnRaw, err := signedTxn.MarshalRLPTo(nil)
	if err != nil {
		return nil, err
	}
	hash, err := e.client.Eth().SendRawTransaction(txnRaw)
	if err != nil {
		return nil, err
	}
	receipt, err := e.waitForReceipt(hash)
	if err != nil {
		return nil, err
	}
	return receipt, nil
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

// DeployDeposit deploys the eth2.0 deposit contract
func (e *depositHandler) deployDeposit() error {
	receipt, err := e.sendTransaction(&ethgo.Transaction{
		Input: deposit.DepositBin(),
	})
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

func (e *depositHandler) GetDepositCount() (uint32, error) {
	contract := e.GetDepositContract()
	count, err := contract.GetDepositCount(ethgo.Latest)
	if err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint32(count), nil
}

// MakeDeposits deposits the minimum required value to become a validator to multiple accounts
func (e *depositHandler) MakeDeposits(accounts []*proto.Account) error {
	errCh := make(chan error, len(accounts))
	for _, acct := range accounts {
		go func(acct *proto.Account) {
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

var depositEvent = abi.MustNewEvent(`event DepositEvent(
	bytes pubkey,
	bytes withdrawal_credentials,
	bytes amount,
	bytes signature,
	bytes index
)`)

// MakeDeposit deposits the minimum required value to become a validator
func (e *depositHandler) MakeDeposit(account *proto.Account) error {
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
	// gas limit must be hardcoded since the estimate gas limit might not be enough with multiple
	// async deposits (small changes in the smart contract change the estimation).
	txn.WithOpts(&contract.TxnOpts{GasLimit: defaultGasLimit, Value: ethgo.Ether(depositAmount)})

	if err := txn.Do(); err != nil {
		return err
	}
	receipt, err := txn.Wait()
	if err != nil {
		return err
	}
	if len(receipt.Logs) != 1 {
		return fmt.Errorf("log not found")
	}

	if _, err := depositEvent.ParseLog(receipt.Logs[0]); err != nil {
		return err
	}
	return nil
}
