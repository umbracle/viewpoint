package server

import (
	"bytes"
	"embed"
	"encoding"
	"fmt"
	"html/template"
	"time"
)

var (
	//go:embed fixtures
	res embed.FS
)

type Config struct {
	Name                 string
	Spec                 *Eth2Spec
	NumTranches          uint64
	NumGenesisValidators uint64
}

func DefaultConfig() *Config {
	return &Config{
		Name:                 "e2e-test",
		Spec:                 DefaultEth2Spec(),
		NumTranches:          1,
		NumGenesisValidators: 1,
	}
}

// Eth2Spec is the config of the Eth2.0 node
type Eth2Spec struct {
	MinGenesisValidatorCount  int
	GenesisDelay              int
	MinGenesisTime            int
	EthFollowDistance         int
	SecondsPerEth1Block       int
	EpochsPerEth1VotingPeriod int
	ShardCommitteePeriod      int
	SlotsPerEpoch             int
	SecondsPerSlot            int
	DepositContract           string
	Altair                    *int
	Bellatrix                 *int
}

func DefaultEth2Spec() *Eth2Spec {
	return &Eth2Spec{
		MinGenesisValidatorCount:  1,
		GenesisDelay:              10,
		MinGenesisTime:            int(time.Now().Add(10 * time.Second).Unix()),
		EthFollowDistance:         1,
		SecondsPerEth1Block:       1,
		EpochsPerEth1VotingPeriod: 64,
		ShardCommitteePeriod:      4,
		SlotsPerEpoch:             32,
		SecondsPerSlot:            12,
	}
}

func (e *Eth2Spec) MarshalText() ([]byte, error) {
	return e.buildConfig(), nil
}

func (e *Eth2Spec) buildConfig() []byte {
	funcMap := template.FuncMap{
		"marshal": func(obj interface{}) string {
			enc, ok := obj.(encoding.TextMarshaler)
			if !ok {
				panic("expected an encoding.TextMarshaler obj")
			}
			res, err := enc.MarshalText()
			if err != nil {
				panic(err)
			}
			return string(res)
		},
		"fork": func(obj interface{}) string {
			num, ok := obj.(*int)
			if !ok {
				panic("BUG: Fork is not an int")
			}
			if num == nil {
				// disable fork
				return "18446744073709551615"
			}
			return fmt.Sprintf("%d", *num)
		},
	}

	tmplFile, err := res.ReadFile("fixtures/config.yaml.tmpl")
	if err != nil {
		panic("file not found")
	}
	tmpl, err := template.New("name").Funcs(funcMap).Parse(string(tmplFile))
	if err != nil {
		panic(fmt.Sprintf("BUG: Failed to load eth2 config template: %v", err))
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, e); err != nil {
		panic(fmt.Sprintf("BUG: Failed to render template: %v", err))
	}
	return tpl.Bytes()
}
