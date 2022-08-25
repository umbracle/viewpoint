package cmd

import (
	"flag"

	"github.com/mitchellh/cli"
	"github.com/umbracle/viewpoint/e2e/framework"
)

// E2ERunCommand is the command to deploy an e2e network
type E2ERunCommand struct {
	UI cli.Ui
}

// Help implements the cli.Command interface
func (c *E2ERunCommand) Help() string {
	return ""
}

// Synopsis implements the cli.Command interface
func (c *E2ERunCommand) Synopsis() string {
	return ""
}

// Run implements the cli.Command interface
func (c *E2ERunCommand) Run(args []string) int {
	flags := flag.NewFlagSet("e2e run", flag.ContinueOnError)

	if err := flags.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	f := framework.New()
	f.Run()

	return 0
}
