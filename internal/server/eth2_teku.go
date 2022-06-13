package server

import (
	"fmt"

	"github.com/umbracle/viewpoint/internal/bls"
	"github.com/umbracle/viewpoint/internal/server/proto"
	"github.com/umbracle/viewpoint/internal/spec"
)

// NewTekuBeacon creates a new teku server
func NewTekuBeacon(config *BeaconConfig) (*spec.Spec, error) {
	cmd := []string{
		// debug log
		"--logging", "debug",
		// eth1x
		"--eth1-endpoint", config.Eth1,
		// run only beacon node
		"--rest-api-enabled",
		// config
		"--network", "/data/config.yaml",
		// initial state
		"--initial-state", "/data/genesis.ssz",
		// port
		"--rest-api-port", `{{ Port "eth2.http" }}`,
		// logs
		"--log-file", "/data/logs.txt",
		"--p2p-advertised-ip", "0.0.0.0",
		"--p2p-port", `{{ Port "eth2.p2p" }}`,
	}
	if config.Bootnode != "" {
		cmd = append(cmd, "--p2p-discovery-bootnodes", config.Bootnode)
	}

	spec := &spec.Spec{}
	spec.WithLabel(proto.NodeClientLabel, proto.NodeClient_Teku.String()).
		WithLabel(proto.NodeTypeLabel, proto.NodeType_Beacon.String()).
		WithContainer("consensys/teku").
		WithTag("22.4.0").
		WithCmd(cmd).
		WithMount("/data").
		WithFile("/data/config.yaml", config.Spec).
		WithFile("/data/genesis.ssz", config.GenesisSSZ).
		WithUser("0:0")

	return spec, nil
}

func NewTekuValidator(config *ValidatorConfig) (*spec.Spec, error) {
	cmd := []string{
		"vc",
		// beacon api
		"--beacon-node-api-endpoint", config.Beacon.GetAddr(proto.NodePortHttp),
		// data
		"--data-path", "/data",
		// config
		"--network", "/data/config.yaml",
		// keys
		"--validator-keys", "/data/keys:/data/pass",
	}
	spec := &spec.Spec{}
	spec.WithLabel(proto.NodeClientLabel, proto.NodeClient_Teku.String()).
		WithLabel(proto.NodeTypeLabel, proto.NodeType_Validator.String()).
		WithContainer("consensys/teku").
		WithTag("22.4.0").
		WithCmd(cmd).
		WithMount("/data").
		WithFile("/data/config.yaml", config.Spec).
		WithUser("0:0")

	for indx, acct := range config.Accounts {
		keystore, err := bls.ToKeystore(acct.Bls, defWalletPassword)
		if err != nil {
			return nil, err
		}

		name := fmt.Sprintf("account_%d", indx)
		spec.WithFile("/data/keys/"+name+".json", keystore).
			WithFile("/data/pass/"+name+".txt", defWalletPassword)
	}
	return spec, nil
}
