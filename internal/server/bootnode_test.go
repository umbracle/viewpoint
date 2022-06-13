package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBootnode(t *testing.T) {
	d, err := NewDocker()
	assert.NoError(t, err)

	bootnode := NewBootnode()

	_, err = d.Deploy(bootnode.Spec)
	assert.NoError(t, err)

	assert.NotEmpty(t, bootnode.Enr)
}
