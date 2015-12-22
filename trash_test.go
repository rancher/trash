package main

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func TestParentPackages(t *testing.T) {
	assert := require.New(t)

	packages := parentPackages("github.com/stretchr/testify/require")
	assert.Len(packages, 4)
	assert.True(packages["github.com"])
	assert.True(packages["github.com/stretchr"])
	assert.True(packages["github.com/stretchr/testify"])
	assert.True(packages["github.com/stretchr/testify/require"])
}
