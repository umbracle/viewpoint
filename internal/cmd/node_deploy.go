package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

// E2ENodeCommand is the command to deploy an e2e network
type NodeDeployCommand struct {
	*Meta

	nodeType      string
	numValidators uint64

	validator   bool
	beaconNode  bool
	withDeposit bool

	count uint64

	repo string
	tag  string
}

// Help implements the cli.Command interface
func (c *NodeDeployCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *NodeDeployCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *NodeDeployCommand) Run(args []string) int {
	flags := c.FlagSet("node deploy")

	flags.StringVar(&c.nodeType, "node-type", "", "")
	flags.Uint64Var(&c.numValidators, "num-validators", 0, "")
	flags.BoolVar(&c.validator, "validator", false, "")
	flags.BoolVar(&c.beaconNode, "beacon-node", false, "")
	flags.BoolVar(&c.withDeposit, "with-deposit", false, "")
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

	var reqJob proto.IsNodeDeployRequest_NodeType
	if c.beaconNode {
		reqJob = &proto.NodeDeployRequest_Beacon_{
			Beacon: &proto.NodeDeployRequest_Beacon{},
		}
	} else if c.validator {
		reqJob = &proto.NodeDeployRequest_Validator_{
			Validator: &proto.NodeDeployRequest_Validator{
				NumValidators: c.numValidators,
				WithDeposit:   c.withDeposit,
			},
		}
	} else {
		c.UI.Output("either --validator or --beacon-node must be set")
	}

	req := &proto.NodeDeployRequest{
		NodeClient: typ,
		Repo:       c.repo,
		Tag:        c.tag,
		NodeType:   reqJob,
	}

	for i := 0; i < int(c.count); i++ {
		if _, err := clt.NodeDeploy(context.Background(), req); err != nil {
			c.UI.Error(err.Error())
			return 1
		}
	}

	return 0
}
