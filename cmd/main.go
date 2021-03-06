package main

import (
	"fmt"
	"os"

	"github.com/mitchellh/cli"
	"github.com/umbracle/viewpoint/internal/cmd"
)

func main() {
	os.Exit(Run(os.Args[1:]))
}

// Run starts the cli
func Run(args []string) int {
	commands := cmd.Commands()

	cli := &cli.CLI{
		Name:     "viewpoint",
		Args:     args,
		Commands: commands,
	}

	exitCode, err := cli.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error executing CLI: %s\n", err.Error())
		return 1
	}

	return exitCode
}
