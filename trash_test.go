package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestParentPackages(t *testing.T) {
	assert := require.New(t)

	packages := parentPackages("", "github.com/stretchr/testify/require")
	assert.Len(packages, 4)
	assert.True(packages["github.com"])
	assert.True(packages["github.com/stretchr"])
	assert.True(packages["github.com/stretchr/testify"])
	assert.True(packages["github.com/stretchr/testify/require"])
}

func TestParseDir(t *testing.T) {
	assert := require.New(t)

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", nil, parser.ImportsOnly)
	if err != nil {
		assert.Fail(err.Error())
		return
	}
	for pkgName, pkg := range pkgs {
		fmt.Printf("pkg: %s\n", pkgName)
		if len(pkg.Files) == 0 {
			fmt.Println("  (no files)")
		}
		for name, f := range pkg.Files {
			fmt.Println("  " + name)
			for _, v := range f.Imports {
				fmt.Println("    " + v.Path.Value[1:len(v.Path.Value)-1])
			}
		}
	}
}

func TestListPackages(t *testing.T) {
	assert := require.New(t)
	p := listPackages("github.com/rancher/trash", "vendor")
	logrus.Debug(p)
	assert.Equal(3, len(p))
	assert.Contains(p, "github.com/rancher/trash")
	assert.Contains(p, "github.com/rancher/trash/util")
	assert.Contains(p, "github.com/rancher/trash/conf")
}
