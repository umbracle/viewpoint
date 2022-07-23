package components

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"text/template"

	"github.com/hashicorp/go-uuid"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/keystore"
	"github.com/umbracle/ethgo/wallet"
	"github.com/umbracle/viewpoint/internal/server/proto"
	"github.com/umbracle/viewpoint/internal/spec"
)

var (
	//go:embed fixtures/genesis.json.tmpl
	genesisTmpl string
)

func NewDevGenesis() (*Eth1Genesis, *wallet.Key, error) {
	key, err := wallet.GenerateKey()
	if err != nil {
		return nil, nil, err
	}
	genesis := &Eth1Genesis{
		Validators:     []ethgo.Address{key.Address()},
		Period:         2,
		MergeForkBlock: 100000,
		TDD:            100000,
	}
	return genesis, key, nil
}

// NewEth1Server creates a new eth1 server with go-ethereum
func NewEth1Server(config *proto.ExecutionConfig) *spec.Spec {
	cmd := []string{
		// init with a custom genesis
		"geth",
		"--datadir", "/data",
		"init", "/data/genesis.json",
		"&&",
		// start the execution node
		"geth",
		"--datadir", "/data",
		"--networkid", "1337",
		"--port", `{{ Port "eth1.p2p" }}`,

		"--http",
		"--http.api", "eth,net,engine,clique",
		"--http.port", `{{ Port "eth1.http" }}`,
		"--authrpc.port", `{{ Port "eth1.authrpc" }}`,
		"--discovery.dns", "\"\"", // disable dns discovery
		// "--verbosity", "4",
	}
	if config.Bootnode != "" {
		cmd = append(cmd, "--bootnodes", config.Bootnode)
	}
	if config.Key != nil {
		// set the node as miner
		minerCmd := []string{
			"--mine",
			"--miner.threads", "1",
			"--miner.etherbase", config.Key.Address().String(),
			"--miner.extradata", config.Key.Address().String(),
			"--miner.gasprice", "16000000000",
			"--unlock", config.Key.Address().String(),
			"--allow-insecure-unlock",
			"--txpool.locals", config.Key.Address().String(),
			"--password", "/data/keystore-password.txt",
		}
		cmd = append(cmd, minerCmd...)
	}

	spec := &spec.Spec{}
	spec.WithName("eth1").
		WithContainer("ethereum/client-go").
		WithTag("v1.10.20").
		WithMount("/data").
		WithFile("/data/genesis.json", config.Genesis).
		WithEntrypoint([]string{"/bin/sh", "-c"}).
		WithCmd([]string{strings.Join(cmd, " ")})

	if config.Key != nil {
		keystore, err := toKeystoreV3(config.Key)
		if err != nil {
			panic(err)
		}
		spec.WithFile("/data/keystore/account.json", string(keystore))
		spec.WithFile("/data/keystore-password.txt", defWalletPassword)
		spec.WithLabel("clique-validator", "true")
	}

	return spec
}

func testHTTPEndpoint(endpoint string) error {
	resp, err := http.Post(endpoint, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

type Eth1Genesis struct {
	MergeForkBlock uint64
	TDD            uint64
	Period         uint64
	Allocs         map[ethgo.Address]string
	Validators     []ethgo.Address
	Extra          string
}

func (e *Eth1Genesis) Build() (string, error) {
	if len(e.Validators) == 0 {
		return "", fmt.Errorf("no genesis validators")
	}

	// build the extra genesis
	sort.Slice(e.Validators, func(i, j int) bool {
		return bytes.Compare(e.Validators[i][:], e.Validators[j][:]) < 0
	})
	extra := make([]byte, 32)
	for _, addr := range e.Validators {
		extra = append(extra, addr.Bytes()...)
	}
	extra = append(extra, make([]byte, 65)...)
	e.Extra = "0x" + hex.EncodeToString(extra)

	tmpl, err := template.New("name").Parse(genesisTmpl)
	if err != nil {
		panic(fmt.Sprintf("BUG: Failed to load eth2 config template: %v", err))
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, e); err != nil {
		panic(fmt.Sprintf("BUG: Failed to render template: %v", err))
	}
	return tpl.String(), nil
}

func toKeystoreV3(key *wallet.Key) ([]byte, error) {
	id, _ := uuid.GenerateUUID()

	// keystore does not include "address" and "id" field
	privKey, err := key.MarshallPrivateKey()
	if err != nil {
		return nil, err
	}
	keystore, err := keystore.EncryptV3(privKey, defWalletPassword)
	if err != nil {
		return nil, err
	}

	var dec map[string]interface{}
	if err := json.Unmarshal(keystore, &dec); err != nil {
		return nil, err
	}
	dec["address"] = key.Address().String()
	dec["uuid"] = id
	dec["id"] = id
	return json.Marshal(dec)
}
