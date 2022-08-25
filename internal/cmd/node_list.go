package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

// E2ENodeListCommand is the command to deploy an e2e network
type NodeListCommand struct {
	*Meta
}

// Help implements the cli.Command interface
func (c *NodeListCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *NodeListCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *NodeListCommand) Run(args []string) int {
	flags := c.FlagSet("node list")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	resp, err := clt.NodeList(context.Background(), &proto.NodeListRequest{})
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	c.UI.Output(formatNodes(resp.Nodes))
	return 0
}

func formatNodes(nodes []*proto.Node) string {
	if len(nodes) == 0 {
		return "No nodes found"
	}

	rows := make([]string, len(nodes)+1)
	rows[0] = "Name|Type|Client"
	for i, d := range nodes {
		rows[i+1] = fmt.Sprintf("%s|%s|%s",
			d.Name,
			d.Type.String(),
			d.Client.String(),
		)
	}
	return formatList(rows)
}
