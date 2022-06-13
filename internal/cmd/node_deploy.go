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

	validator  bool
	beaconNode bool
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

	if c.beaconNode {
		if _, err := clt.DeployNode(context.Background(), &proto.DeployNodeRequest{NodeClient: typ}); err != nil {
			c.UI.Error(err.Error())
			return 1
		}
	} else if c.validator {
		if _, err := clt.DeployValidator(context.Background(), &proto.DeployValidatorRequest{NodeClient: typ, NumValidators: c.numValidators}); err != nil {
			c.UI.Error(err.Error())
			return 1
		}
	} else {
		c.UI.Output("either --validator or --beacon-node must be set")
	}
	return 0
}
