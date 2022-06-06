package cmd

import (
	"flag"
	"fmt"
	"os"

	"github.com/mitchellh/cli"
	"github.com/umbracle/viewpoint/internal/cmd/server"
	"github.com/umbracle/viewpoint/internal/server/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Commands returns the cli commands
func Commands() map[string]cli.CommandFactory {
	ui := &cli.BasicUi{
		Reader:      os.Stdin,
		Writer:      os.Stdout,
		ErrorWriter: os.Stderr,
	}

	meta := &Meta{
		UI: ui,
	}

	return map[string]cli.CommandFactory{
		"server": func() (cli.Command, error) {
			return &server.Command{
				UI: ui,
			}, nil
		},
		"node": func() (cli.Command, error) {
			return &NodeCommand{
				Meta: meta,
			}, nil
		},
		"version": func() (cli.Command, error) {
			return &VersionCommand{
				UI: ui,
			}, nil
		},
	}
}

type Meta struct {
	UI   cli.Ui
	addr string
}

func (m *Meta) FlagSet(n string) *flag.FlagSet {
	f := flag.NewFlagSet(n, flag.ContinueOnError)
	f.StringVar(&m.addr, "address", "localhost:5555", "Address of the http api")
	return f
}

// Conn returns a grpc connection
func (m *Meta) Conn() (proto.E2EServiceClient, error) {
	conn, err := grpc.Dial(m.addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %v", err)
	}
	clt := proto.NewE2EServiceClient(conn)
	return clt, nil
}
