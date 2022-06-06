package proto

type DepositData struct {
	Pubkey                []byte   `json:"pubkey" ssz-size:"48"`
	WithdrawalCredentials []byte   `json:"withdrawal_credentials" ssz-size:"32"`
	Amount                uint64   `json:"amount"`
	Signature             []byte   `json:"signature" ssz-size:"96"`
	Root                  [32]byte `ssz:"-"`
}

type DepositMessage struct {
	Pubkey                []byte `json:"pubkey" ssz-size:"48"`
	WithdrawalCredentials []byte `json:"withdrawal_credentials" ssz-size:"32"`
	Amount                uint64 `json:"amount"`
}

type SigningData struct {
	ObjectRoot []byte `ssz-size:"32"`
	Domain     []byte `ssz-size:"32"`
}

type ForkData struct {
	CurrentVersion        []byte `ssz-size:"4"`
	GenesisValidatorsRoot []byte `ssz-size:"32"`
}
