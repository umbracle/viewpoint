package components

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/viewpoint/internal/docker"
)

func TestEth1_Multiple(t *testing.T) {
	// t.Skip()

	d, err := docker.NewDocker()
	assert.NoError(t, err)

	// test that multiple eth1 nodes are deployed and
	// get assigned a different port
	srv1, err := d.Deploy(NewEth1Server())
	assert.NoError(t, err)
	//defer srv1.Stop()
	fmt.Println(srv1)

	/*
		srv2, err := d.Deploy(NewEth1Server())
		assert.NoError(t, err)
		defer srv2.Stop()

		addr1 := srv1.GetAddr(proto.NodePortEth1Http)
		addr2 := srv2.GetAddr(proto.NodePortEth1Http)
		assert.NotEqual(t, addr1, addr2)
	*/
}
