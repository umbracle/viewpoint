package server

import (
	"testing"

	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestServer_Simple(t *testing.T) {
	config := DefaultConfig()
	srv, err := NewServer(hclog.NewNullLogger(), config)
	assert.NoError(t, err)
	defer srv.Stop()

	assert.Len(t, srv.tranches, 1)
}
