package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/viewpoint/internal/docker"
)

func TestBootnode(t *testing.T) {
	d, err := docker.NewDocker()
	assert.NoError(t, err)

	bootnode := NewBootnode()

	_, err = d.Deploy(bootnode.Spec)
	assert.NoError(t, err)

	assert.NotEmpty(t, bootnode.Enr)
}
