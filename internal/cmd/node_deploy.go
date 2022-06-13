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

	var req *proto.NodeDeployRequest
	if c.beaconNode {
		req = &proto.NodeDeployRequest{
			NodeClient: typ,
			NodeType: &proto.NodeDeployRequest_Beacon_{
				Beacon: &proto.NodeDeployRequest_Beacon{},
			},
		}
	} else if c.validator {
		req = &proto.NodeDeployRequest{
			NodeClient: typ,
			NodeType: &proto.NodeDeployRequest_Validator_{
				Validator: &proto.NodeDeployRequest_Validator{
					NumValidators: c.numValidators,
					WithDeposit:   c.withDeposit,
				},
			},
		}
	} else {
		c.UI.Output("either --validator or --beacon-node must be set")
	}

	for i := 0; i < int(c.count); i++ {
		if _, err := clt.NodeDeploy(context.Background(), req); err != nil {
			c.UI.Error(err.Error())
			return 1
		}
	}

	return 0
}
