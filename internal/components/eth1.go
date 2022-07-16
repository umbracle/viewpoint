package components

import (
	"fmt"
	"net/http"
	"strings"

	specX "github.com/umbracle/viewpoint/internal/spec"
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
