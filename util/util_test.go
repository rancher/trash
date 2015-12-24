package util

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestTrue(t *testing.T) {
	assert := require.New(t)
	assert.True(true)
}
