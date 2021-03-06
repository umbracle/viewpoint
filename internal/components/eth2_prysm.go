package components

import (
	"encoding/json"
	"strings"

	"github.com/umbracle/ethgo/keystore"
	"github.com/umbracle/go-eth-consensus/bls"
	"github.com/umbracle/viewpoint/internal/server/proto"
	"github.com/umbracle/viewpoint/internal/spec"
)

// NewPrysmBeacon creates a new prysm server
func NewPrysmBeacon(config *proto.BeaconConfig) (*spec.Spec, error) {
	cmd := []string{
		"--verbosity", "debug",
		// eth1x
		"--http-web3provider", config.Eth1,
		"--contract-deployment-block", "0",
		// these sync fields have to be disabled for single node
		"--min-sync-peers", "1",
		// grpc endpoint
		"--rpc-host", "0.0.0.0",
		"--rpc-port", `{{ Port "eth2.prysm.grpc" }}`,
		// http gateway for the grpc server and Open spec http server
		"--grpc-gateway-host", "0.0.0.0",
		"--grpc-gateway-port", `{{ Port "eth2.http" }}`,
		// config
		"--chain-config-file", "/data/config.yaml",
		"--genesis-state", "/data/genesis.ssz",
		// accept terms and conditions
		"--accept-terms-of-use",
		// use data dir
		"--datadir", "/data/eth2",
		"--force-clear-db",
		// other
		"--minimum-peers-per-subnet", "0",
		// p2p port
		"--p2p-tcp-port", `{{ Port "eth2.p2p" }}`,
		"--p2p-udp-port", `{{ Port "eth2.p2p" }}`,
	}
	if config.Bootnode != "" {
		cmd = append(cmd, "--bootstrap-node", config.Bootnode)
	}
	spec := &spec.Spec{}
	spec.WithLabel(proto.NodeClientLabel, proto.NodeClient_Prysm.String()).
		WithLabel(proto.NodeTypeLabel, proto.NodeType_Beacon.String()).
		WithContainer("gcr.io/prysmaticlabs/prysm/beacon-chain").
		WithTag("v2.0.6").
		WithCmd(cmd).
		WithMount("/data").
		WithFile("/data/config.yaml", config.Spec).
		WithFile("/data/genesis.ssz", config.GenesisSSZ)

	return spec, nil
}

const defWalletPassword = "qwerty"

func NewPrysmValidator(config *proto.ValidatorConfig) (*spec.Spec, error) {
	store := &accountStore{}
	for _, acct := range config.Accounts {
		store.AddKey(acct.Bls)
	}

	keystore, err := store.ToKeystore(defWalletPassword)
	if err != nil {
		return nil, err
	}

	cmd := []string{
		"--verbosity", "debug",
		// accept terms and conditions
		"--accept-terms-of-use",
		// wallet dir and password
		"--wallet-dir", "/data",
		"--wallet-password-file", "/data/wallet-password.txt",
		// beacon node reference of the GRPC endpoint
		"--beacon-rpc-provider", strings.TrimPrefix(config.Beacon.GetAddr(proto.NodePortPrysmGrpc), "http://"),
		// config
		"--chain-config-file", "/data/config.yaml",
	}
	spec := &spec.Spec{}
	spec.WithLabel(proto.NodeClientLabel, proto.NodeClient_Prysm.String()).
		WithLabel(proto.NodeTypeLabel, proto.NodeType_Validator.String()).
		WithContainer("gcr.io/prysmaticlabs/prysm/validator").
		WithTag("v2.0.6").
		WithCmd(cmd).
		WithMount("/data").
		WithFile("/data/direct/accounts/all-accounts.keystore.json", keystore).
		WithFile("/data/wallet-password.txt", defWalletPassword).
		WithFile("/data/config.yaml", config.Spec)

	return spec, nil
}

// accountStore is the format used by all managers??
type accountStore struct {
	PrivateKeys [][]byte `json:"private_keys"`
	PublicKeys  [][]byte `json:"public_keys"`
}

func (a *accountStore) AddKey(k *bls.Key) error {
	if a.PrivateKeys == nil {
		a.PrivateKeys = [][]byte{}
	}
	if a.PublicKeys == nil {
		a.PublicKeys = [][]byte{}
	}

	priv, err := k.Prv.Marshal()
	if err != nil {
		return err
	}
	pub := k.Pub.Serialize()

	a.PrivateKeys = append(a.PrivateKeys, priv)
	a.PublicKeys = append(a.PublicKeys, pub[:])

	return nil
}

func (a *accountStore) ToKeystore(password string) ([]byte, error) {
	raw, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	keystore, err := keystore.EncryptV4(raw, password)
	if err != nil {
		return nil, err
	}
	return keystore, nil
}
