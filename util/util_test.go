package util

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func TestTrue(t *testing.T) {
	assert := require.New(t)
	assert.True(true)
}
