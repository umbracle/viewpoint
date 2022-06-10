package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBootnode(t *testing.T) {
	d, err := NewDocker()
	assert.NoError(t, err)

	b, err := NewBootnode(d)
	assert.NoError(t, err)
	assert.NotEmpty(t, b.Enr)
}
