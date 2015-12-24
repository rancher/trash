package util

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTrue(t *testing.T) {
	assert := require.New(t)
	assert.True(true)
}

func TestOneMsg(t *testing.T) {
	assert := require.New(t)
	c := OneOff("qq")

	s, ok := <-c
	assert.Equal("qq", s)
	assert.True(ok)

	s, ok = <-c
	assert.False(ok)
}
