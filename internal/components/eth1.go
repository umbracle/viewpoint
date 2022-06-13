package components

import (
	"net/http"

	"github.com/umbracle/viewpoint/internal/server/proto"
	specX "github.com/umbracle/viewpoint/internal/spec"
)

// NewEth1Server creates a new eth1 server with go-ethereum
func NewEth1Server() *specX.Spec {
	cmd := []string{
		"--dev",
		"--dev.period", "1",
		"--http", "--http.addr", "0.0.0.0",
		"--http.port", `{{ Port "eth1.http" }}`,
	}
	spec := &specX.Spec{}
	spec.WithName("eth1").
		WithContainer("ethereum/client-go").
		WithTag("v1.9.25").
		WithCmd(cmd).
		WithRetry(func(n specX.Node) error {
			return testHTTPEndpoint(n.GetAddr(proto.NodePortEth1Http))
		})
	return spec
}

func testHTTPEndpoint(endpoint string) error {
	resp, err := http.Post(endpoint, "application/json", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
