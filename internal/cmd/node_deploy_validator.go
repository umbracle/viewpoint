package cmd

import (
	"context"
	"fmt"

	"github.com/umbracle/viewpoint/internal/server/proto"
)

// E2ENodeCommand is the command to deploy an e2e network
type NodeDeployValidatorCommand struct {
	*Meta

	nodeType      string
	numValidators uint64

	withBeacon bool

	trancheNum  uint64
	beaconCount uint64

	repo string
	tag  string
}

// Help implements the cli.Command interface
func (c *NodeDeployValidatorCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *NodeDeployValidatorCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *NodeDeployValidatorCommand) Run(args []string) int {
	flags := c.FlagSet("node deploy")

	flags.StringVar(&c.nodeType, "type", "", "")
	flags.Uint64Var(&c.numValidators, "num-validators", 0, "")
	flags.Uint64Var(&c.trancheNum, "tranche", 0, "")
	flags.BoolVar(&c.withBeacon, "beacon", false, "")
	flags.Uint64Var(&c.beaconCount, "beacon-count", 1, "")
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

	if c.beaconCount == 0 {
		c.UI.Error("--count cannot be zero")
		return 1
	}

	reqJob := &proto.NodeDeployRequest_Validator_{
		Validator: &proto.NodeDeployRequest_Validator{
			NumValidators: c.numValidators,
			NumTranch:     c.trancheNum,
			WithBeacon:    c.withBeacon,
			BeaconCount:   c.beaconCount,
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
