package cmd

import (
	"context"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

// E2EValidatorDepositCommand is the command to deploy an e2e network
type DepositCommand struct {
	*Meta

	numValidators uint64
	withDeposit   bool
}

// Help implements the cli.Command interface
func (c *DepositCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *DepositCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *DepositCommand) Run(args []string) int {
	flags := c.FlagSet("deposit")

	flags.Uint64Var(&c.numValidators, "num-validators", 0, "")
	flags.BoolVar(&c.withDeposit, "with-deposit", false, "")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	req := &proto.DepositRequest{
		NumValidators: c.numValidators,
		WithDeposit:   c.withDeposit,
	}
	resp, err := clt.Deposit(context.Background(), req)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.UI.Output(resp.TranchPath)
	return 0
}
