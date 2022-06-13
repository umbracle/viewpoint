package server

// NewEth1Server creates a new eth1 server with go-ethereum
func NewEth1Server() *Spec {
	cmd := []string{
		"--dev",
		"--dev.period", "1",
		"--http", "--http.addr", "0.0.0.0",
		"--http.port", `{{ Port "eth1.http" }}`,
	}
	spec := &Spec{}
	spec.WithName("eth1").
		WithContainer("ethereum/client-go").
		WithTag("v1.9.25").
		WithCmd(cmd).
		WithRetry(func(n *node) error {
			return testHTTPEndpoint(n.GetAddr(NodePortEth1Http))
		})
	return spec
}
