package components

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/umbracle/viewpoint/internal/docker"
)

func TestBootnode(t *testing.T) {
	d, err := docker.NewDocker()
	assert.NoError(t, err)

	t.Run("V5", func(t *testing.T) {
		bootnode := NewBootnodeV5()

		_, err = d.Deploy(bootnode.Spec)
		assert.NoError(t, err)

		assert.NotEmpty(t, bootnode.Enr)
	})

	t.Run("V4", func(t *testing.T) {
		bootnode := NewBootnodeV4()

		_, err = d.Deploy(bootnode.Spec)
		assert.NoError(t, err)

		assert.NotEmpty(t, bootnode.Enode)
	})
}
