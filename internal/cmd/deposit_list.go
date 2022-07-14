package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

// E2EValidatorDepositCommand is the command to deploy an e2e network
type DepositListCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *DepositListCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *DepositListCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *DepositListCommand) Run(args []string) int {
	flags := c.FlagSet("deposit list")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	resp, err := clt.DepositList(context.Background(), &proto.DepositListRequest{})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	c.UI.Output(formatTranches(resp.Tranches))
	return 0
}

func formatTranches(tranches []*proto.TrancheStub) string {
	if len(tranches) == 0 {
		return "No tranches found"
	}

	rows := make([]string, len(tranches)+1)
	rows[0] = "Index|Validator|Num accounts"
	for i, d := range tranches {
		name := d.Name
		if name == "" {
			name = "-"
		}
		rows[i+1] = fmt.Sprintf("%d|%s|%d",
			d.Index,
			d.Name,
			len(d.Accounts),
		)
	}
	return formatList(rows)
}
