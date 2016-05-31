package util

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOneMsg(t *testing.T) {
	assert := require.New(t)
	c := OneStr("qq")

	s, ok := <-c
	assert.Equal("qq", s)
	assert.True(ok)

	s, ok = <-c
	assert.False(ok)
}
