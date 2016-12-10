package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

func TestCollectImports(t *testing.T) {
	defer logrus.SetLevel(logrus.GetLevel())
	logrus.SetLevel(logrus.FatalLevel)

	assert := require.New(t)
	os.Chdir("./test")
	defer os.Chdir("..")

	libRoot := path.Join(os.Getenv("GOPATH"), "src")
	trashCache := path.Join(os.Getenv("HOME"), ".trash-cache")

	// Test that build tags are ignored
	allPackages := collectImports(".", libRoot, trashCache, []string{})
	assert.Equal(2, len(allPackages))
	assert.Contains(allPackages, "github.com/Sirupsen/logrus")

	// Test that a build tag can be used to filter
	filteredPackages := collectImports(".", libRoot, trashCache, []string{"ignore"})
	assert.Equal(1, len(filteredPackages))
	assert.NotContains(filteredPackages, "github.com/Sirupsen/logrus")
}

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
	assert.Equal(4, len(p))
	assert.Contains(p, "github.com/rancher/trash")
	assert.Contains(p, "github.com/rancher/trash/util")
	assert.Contains(p, "github.com/rancher/trash/conf")
	assert.Contains(p, "github.com/rancher/trash/test")
}
