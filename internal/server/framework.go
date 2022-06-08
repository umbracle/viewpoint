package server

import "github.com/umbracle/viewpoint/internal/server/proto"

type ValidatorConfig struct {
	Spec     *Eth2Spec
	Accounts []*proto.Account
	Beacon   *node
}

type BeaconConfig struct {
	Spec       *Eth2Spec
	Eth1       string
	Bootnode   string
	GenesisSSZ []byte
}

type CreateBeacon2 func(cfg *BeaconConfig) ([]nodeOption, error)

// CreateBeacon is a factory method to create beacon nodes
type CreateBeacon func(cfg *BeaconConfig) (*node, error)

type CreateValidator2 func(cfg *ValidatorConfig) ([]nodeOption, error)

// CreateValidator is a factory method to create validator nodes
type CreateValidator func(cfg *ValidatorConfig) (*node, error)
