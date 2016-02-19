package main

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParentPackages(t *testing.T) {
	assert := require.New(t)

	packages := parentPackages("github.com/rancher/trash", "github.com/rancher/trash/vendor/github.com/stretchr/testify/require")
	assert.Len(packages, 4)
	assert.True(packages["github.com/rancher/trash/vendor/github.com"])
	assert.True(packages["github.com/rancher/trash/vendor/github.com/stretchr"])
	assert.True(packages["github.com/rancher/trash/vendor/github.com/stretchr/testify"])
	assert.True(packages["github.com/rancher/trash/vendor/github.com/stretchr/testify/require"])
}
