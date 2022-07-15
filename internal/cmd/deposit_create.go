package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

// E2EValidatorDepositCommand is the command to deploy an e2e network
type DepositCreateCommand struct {
	*Meta

	numValidators uint64
}

// Help implements the cli.Command interface
func (c *DepositCreateCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *DepositCreateCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *DepositCreateCommand) Run(args []string) int {
	flags := c.FlagSet("deposit create")

	flags.Uint64Var(&c.numValidators, "num-validators", 0, "")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	req := &proto.DepositCreateRequest{
		NumValidators: c.numValidators,
	}
	resp, err := clt.DepositCreate(context.Background(), req)
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.UI.Output(formatTranche(resp.Tranche))
	return 0
}

func formatTranche(tranche *proto.TrancheStub) string {
	name := tranche.Name
	if name == "" {
		name = "-"
	}
	base := formatKV([]string{
		fmt.Sprintf("Index|%d", tranche.Index),
		fmt.Sprintf("Path|%s", tranche.Path),
		fmt.Sprintf("Validator|%s", name),
		fmt.Sprintf("Num accounts|%d", len(tranche.Accounts)),
	})
	return base
}
