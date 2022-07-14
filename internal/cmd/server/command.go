package server

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/mitchellh/cli"
	"github.com/umbracle/viewpoint/internal/server"
)

// Command is the command that starts the agent
type Command struct {
	UI       cli.Ui
	client   *server.Server
	logLevel string
}

// Help implements the cli.Command interface
func (c *Command) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *Command) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *Command) Run(args []string) int {
	config, err := c.readConfig(args)
	if err != nil {
		c.UI.Output(fmt.Sprintf("failed to read config: %v", err))
		return 1
	}

	logger := hclog.New(&hclog.LoggerOptions{
		Name:  "viewpoint",
		Level: hclog.LevelFromString(c.logLevel),
	})
	client, err := server.NewServer(logger, config)
	if err != nil {
		c.UI.Output(fmt.Sprintf("failed to start server: %v", err))
		return 1
	}
	c.client = client
	return c.handleSignals()
}

func (c *Command) handleSignals() int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	sig := <-signalCh

	c.UI.Output(fmt.Sprintf("Caught signal: %v", sig))
	c.UI.Output("Gracefully shutting down agent...")

	gracefulCh := make(chan struct{})
	go func() {
		c.client.Stop()
		close(gracefulCh)
	}()

	select {
	case <-signalCh:
		return 1
	case <-time.After(10 * time.Second):
		return 1
	case <-gracefulCh:
		return 0
	}
}

func (c *Command) readConfig(args []string) (*server.Config, error) {
	var name, genesisTime string
	var genesisValidatorCount, numTranches uint64

	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.Usage = func() { c.UI.Error(c.Help()) }

	flags.StringVar(&name, "name", "random", "")
	flags.Uint64Var(&genesisValidatorCount, "genesis-validator-count", 10, "")
	flags.StringVar(&genesisTime, "genesis-time", "", "")
	flags.Uint64Var(&numTranches, "num-tranches", 1, "")

	if err := flags.Parse(args); err != nil {
		return nil, err
	}

	config := server.DefaultConfig()
	config.Name = name
	config.Spec.GenesisValidatorCount = int(genesisValidatorCount)

	config.Spec.MinGenesisTime = int(time.Now().Unix())
	if genesisTime != "" {
		duration, err := time.ParseDuration(genesisTime)
		if err != nil {
			return nil, fmt.Errorf("failed to parse genesis time: %v", err)
		}
		config.Spec.MinGenesisTime += int(duration.Seconds())
	}
	return config, nil
}
