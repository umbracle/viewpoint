package components

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/umbracle/viewpoint/internal/spec"
)

var (
	bootnodeRegexp = regexp.MustCompile("\"Running bootnode: enr:(.*)\"")
)

type Bootnode struct {
	*spec.Spec

	Enr string
}

func NewBootnodeV5() *Bootnode {
	decodeEnr := func(node spec.Node) (string, error) {
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
		"--discv5-port", `{{ Port "eth.bootnode" }}`,
	}

	b := &Bootnode{}

	ss := &spec.Spec{}
	ss.WithName("v5-bootnode").
		WithCmd(cmd).
		WithContainer("gcr.io/prysmaticlabs/prysm/bootnode").
		WithRetry(func(n spec.Node) error {
			enr, err := decodeEnr(n)
			if err != nil {
				return err
			}
			b.Enr = enr
			return nil
		})

	b.Spec = ss
	return b
}

type BootnodeV4 struct {
	*spec.Spec

	Enode string
}

func NewBootnodeV4() *BootnodeV4 {
	cmd := []string{
		// init with a custom genesis
		"bootnode",
		"--genkey", "boot.key",
		"&&",
		// start the bootnode
		"bootnode",
		"--nodekey", "boot.key",
		"--addr", `:{{ Port "eth.bootnode" }}`,
		"--verbosity", "9",
	}

	b := &BootnodeV4{}

	ss := &spec.Spec{}
	ss.WithName("v4-bootnode").
		WithContainer("ethereum/client-go").
		WithTag("alltools-release-1.10").
		WithEntrypoint([]string{"/bin/sh", "-c"}).
		WithCmd([]string{strings.Join(cmd, " ")}).
		WithRetry(func(n spec.Node) error {
			logs, err := n.GetLogs()
			if err != nil {
				return err
			}
			lines := strings.Split(logs, "\n")
			if len(lines) == 0 {
				return fmt.Errorf("not ready")
			}
			b.Enode = lines[0]
			return nil
		})

	b.Spec = ss
	return b
}
