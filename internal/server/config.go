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
	Name string
	Spec *Eth2Spec
}

func DefaultConfig() *Config {
	return &Config{
		Name: "e2e-test",
		Spec: &Eth2Spec{},
	}
}

// Eth2Spec is the config of the Eth2.0 node
type Eth2Spec struct {
	GenesisValidatorCount     int
	GenesisDelay              int
	MinGenesisTime            int
	EthFollowDistance         int
	SecondsPerEth1Block       int
	EpochsPerEth1VotingPeriod int
	ShardCommitteePeriod      int
	SlotsPerEpoch             int
	SecondsPerSlot            int
	DepositContract           string
	Forks                     Forks
}

type Forks struct {
	Altair    *int
	Bellatrix *int
}

func (e *Eth2Spec) MarshalText() ([]byte, error) {
	return e.buildConfig(), nil
}

func (e *Eth2Spec) buildConfig() []byte {
	// set up default config values
	if e.GenesisValidatorCount == 0 {
		e.GenesisValidatorCount = 1
	}
	if e.GenesisDelay == 0 {
		e.GenesisDelay = 10 // second
	}
	if e.MinGenesisTime == 0 {
		e.MinGenesisTime = int(time.Now().Unix())
	}
	if e.EthFollowDistance == 0 {
		e.EthFollowDistance = 1 // blocks
	}
	if e.SecondsPerEth1Block == 0 {
		e.SecondsPerEth1Block = 1 // second
	}
	if e.EpochsPerEth1VotingPeriod == 0 {
		e.EpochsPerEth1VotingPeriod = 64
	}
	if e.ShardCommitteePeriod == 0 {
		e.ShardCommitteePeriod = 4
	}
	if e.SlotsPerEpoch == 0 {
		e.SlotsPerEpoch = 12 // default 32 slots
	}
	if e.SecondsPerSlot == 0 {
		e.SecondsPerSlot = 3 // default 12 seconds
	}

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
