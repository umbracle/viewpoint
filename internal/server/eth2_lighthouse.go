package server

import (
	"encoding/hex"

	"github.com/umbracle/viewpoint/internal/bls"
	"github.com/umbracle/viewpoint/internal/server/proto"
	"github.com/umbracle/viewpoint/internal/spec"
)

// NewLighthouseBeacon creates a new prysm server
func NewLighthouseBeacon(config *BeaconConfig) (*spec.Spec, error) {
	cmd := []string{
		"lighthouse", "beacon_node",
		"--http", "--http-address", "0.0.0.0",
		"--http-port", `{{ Port "eth2.http" }}`,
		"--eth1-endpoints", config.Eth1,
		"--target-peers", "1",
		"--testnet-dir", "/data",
		"--http-allow-sync-stalled",
		"--debug-level", "trace",
		"--subscribe-all-subnets",
		"--staking",
		"--port", `{{ Port "eth2.p2p" }}`,
		"--enr-address", "127.0.0.1",
		"--enr-udp-port", `{{ Port "eth2.p2p" }}`,
		"--enr-tcp-port", `{{ Port "eth2.p2p" }}`,
		// required to allow discovery in private networks
		"--disable-packet-filter",
		"--enable-private-discovery",
	}
	spec := &spec.Spec{}
	spec.WithLabel(proto.NodeClientLabel, proto.NodeClient_Lighthouse.String()).
		WithLabel(proto.NodeTypeLabel, proto.NodeType_Beacon.String()).
		WithContainer("sigp/lighthouse").
		WithTag("v2.2.1").
		WithCmd(cmd).
		WithMount("/data").
		WithFile("/data/config.yaml", config.Spec).
		WithFile("/data/genesis.ssz", config.GenesisSSZ).
		WithFile("/data/deploy_block.txt", "0")

	if config.Bootnode != "" {
		spec.WithFile("/data/boot_enr.yaml", "- "+config.Bootnode+"\n")
	}
	return spec, nil
}

func NewLighthouseValidator(config *ValidatorConfig) (*spec.Spec, error) {
	cmd := []string{
		"lighthouse", "vc",
		"--debug-level", "debug",
		"--datadir", "/data/node",
		"--beacon-nodes", config.Beacon.GetAddr(proto.NodePortHttp),
		"--testnet-dir", "/data",
		"--init-slashing-protection",
	}
	spec := &spec.Spec{}
	spec.WithLabel(proto.NodeClientLabel, proto.NodeClient_Lighthouse.String()).
		WithLabel(proto.NodeTypeLabel, proto.NodeType_Validator.String()).
		WithContainer("sigp/lighthouse").
		WithTag("v2.2.1").
		WithCmd(cmd).
		WithMount("/data").
		WithFile("/data/config.yaml", config.Spec).
		WithFile("/data/deploy_block.txt", "0")

	// append validators
	for _, acct := range config.Accounts {
		pub := acct.Bls.PubKey()
		pubStr := "0x" + hex.EncodeToString(pub[:])

		keystore, err := bls.ToKeystore(acct.Bls, defWalletPassword)
		if err != nil {
			return nil, err
		}

		spec.WithFile("/data/node/validators/"+pubStr+"/voting-keystore.json", keystore).
			WithFile("/data/node/secrets/"+pubStr, defWalletPassword)
	}
	return spec, nil
}
