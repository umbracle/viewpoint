package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEth1_Multiple(t *testing.T) {
	// test that multiple eth1 nodes are deployed and
	// get assigned a different port
	srv1, err := newNode(NewEth1Server()...)
	assert.NoError(t, err)

	srv2, err := newNode(NewEth1Server()...)
	assert.NoError(t, err)

	addr1 := srv1.GetAddr(NodePortEth1Http)
	addr2 := srv2.GetAddr(NodePortEth1Http)
	assert.NotEqual(t, addr1, addr2)
}
