package server

import (
	"fmt"
	"regexp"

	specX "github.com/umbracle/viewpoint/internal/spec"
)

var (
	bootnodeRegexp = regexp.MustCompile("\"Running bootnode: enr:(.*)\"")
)

type Bootnode struct {
	*specX.Spec

	Enr string
}

func NewBootnode() *Bootnode {
	decodeEnr := func(node specX.Node) (string, error) {
		logs, err := node.GetLogs()
		if err != nil {
			return "", err
		}
		match := bootnodeRegexp.FindStringSubmatch(logs)
		if len(match) == 0 {
			// not found
			return "", fmt.Errorf("not found")
		} else {
			return "enr:" + match[1], nil
		}
	}

	cmd := []string{
		"--debug",
		"--external-ip", "127.0.0.1",
		"--discv5-port", "3000",
	}

	b := &Bootnode{}

	spec := &specX.Spec{}
	spec.WithName("bootnode").
		WithCmd(cmd).
		WithContainer("gcr.io/prysmaticlabs/prysm/bootnode").
		WithRetry(func(n specX.Node) error {
			enr, err := decodeEnr(n)
			if err != nil {
				return err
			}
			b.Enr = enr
			return nil
		})

	b.Spec = spec
	return b
}
