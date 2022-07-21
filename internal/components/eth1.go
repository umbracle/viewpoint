package components

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"text/template"

	"github.com/umbracle/ethgo"
	specX "github.com/umbracle/viewpoint/internal/spec"
)

var (
	//go:embed fixtures/genesis.json.tmpl
	genesisTmpl string
)

var genesis = `{
	"config": {
	  "chainId": 1337,
	  "homesteadBlock": 0,
	  "eip150Block": 0,
	  "eip155Block": 0,
	  "eip158Block": 0,
	  "byzantiumBlock": 0,
	  "constantinopleBlock": 0,
	  "petersburgBlock": 0,
	  "istanbulBlock": 0,
	  "berlinBlock": 0,
	  "londonBlock": 0,
	  "mergeForkBlock": 308,
	  "terminalTotalDifficulty": 616,
	  "clique": {
		"period": 2,
		"epoch": 30000
	  }
	},
	"alloc": {
	  "0x878705ba3f8bc32fcf7f4caa1a35e72af65cf766": {"balance": "100000000000000000000000000000"}
	},
	"coinbase" : "0x0000000000000000000000000000000000000000",
	"difficulty": "1",
	"extradata": "0x0000000000000000000000000000000000000000000000000000000000000000878705ba3f8bc32fcf7f4caa1a35e72af65cf7660000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000",
	"gasLimit" : "0xffffff",
	"nonce" : "0x0000000000000042",
	"mixhash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
	"parentHash" : "0x0000000000000000000000000000000000000000000000000000000000000000",
	"timestamp" : "0x00"
}`

// NewEth1Server creates a new eth1 server with go-ethereum
func NewEth1Server() *specX.Spec {
	bashCmd := []string{
		"geth init /data/genesis.json --datadir /data && geth --datadir /data",
	}

	fmt.Println(strings.Join(bashCmd, " "))

	spec := &specX.Spec{}
	spec.WithName("eth1").
		WithContainer("ethereum/client-go").
		WithTag("v1.9.25").
		WithMount("/data").
		WithFile("/data/genesis.json", genesis).
		WithEntrypoint([]string{"/bin/sh", "-c"}).
		WithCmd(bashCmd)

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
