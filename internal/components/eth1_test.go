package components

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/ethgo"
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

func TestEth1_BuildGenesis(t *testing.T) {

	e := &Eth1Genesis{
		Allocs: map[ethgo.Address]string{
			{}:    "10000000000",
			{0x1}: "10000000000",
		},
		Validators: []ethgo.Address{
			{0x1},
			{0x2},
			{0x3},
		},
	}
	fmt.Println(e.Build())
}
