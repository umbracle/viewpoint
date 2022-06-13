package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

// E2ENodeStatusCommand is the command to deploy an e2e network
type NodeStatusCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *NodeStatusCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *NodeStatusCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *NodeStatusCommand) Run(args []string) int {
	flags := c.FlagSet("node status")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	args = flags.Args()
	if len(args) != 1 {
		c.UI.Error("expected one argument")
		return 1
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	resp, err := clt.NodeStatus(context.Background(), &proto.NodeStatusRequest{Name: args[0]})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.UI.Output(formatNode(resp.Node))
	return 0
}

func formatNode(node *proto.Node) string {
	base := formatKV([]string{
		fmt.Sprintf("Name|%s", node.Name),
		fmt.Sprintf("Type|%s", node.Type.String()),
		fmt.Sprintf("Client|%s", node.Client.String()),
	})
	return base
}
