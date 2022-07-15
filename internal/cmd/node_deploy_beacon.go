package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

// E2ENodeCommand is the command to deploy an e2e network
type NodeDeployBeaconCommand struct {
	*Meta

	count uint64

	nodeType string
	repo     string
	tag      string
}

// Help implements the cli.Command interface
func (c *NodeDeployBeaconCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *NodeDeployBeaconCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *NodeDeployBeaconCommand) Run(args []string) int {
	flags := c.FlagSet("node deploy")

	flags.StringVar(&c.nodeType, "type", "", "")
	flags.Uint64Var(&c.count, "count", 1, "")
	flags.StringVar(&c.repo, "repo", "", "")
	flags.StringVar(&c.tag, "tag", "", "")

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	clt, err := c.Conn()
	if err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	typ, ok := proto.StringToNodeClient(c.nodeType)
	if !ok {
		c.UI.Error(fmt.Sprintf("node type %s not found", c.nodeType))
		return 1
	}

	if c.count == 0 {
		c.UI.Error("--count cannot be zero")
		return 1
	}

	reqJob := &proto.NodeDeployRequest_Beacon_{
		Beacon: &proto.NodeDeployRequest_Beacon{
			Count: c.count,
		},
	}

	req := &proto.NodeDeployRequest{
		NodeClient: typ,
		Repo:       c.repo,
		Tag:        c.tag,
		NodeType:   reqJob,
	}

	if _, err := clt.NodeDeploy(context.Background(), req); err != nil {
		c.UI.Error(err.Error())
		return 1
	}
	return 0
}
