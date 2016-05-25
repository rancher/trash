package conf

import (
	yaml "github.com/cloudfoundry-incubator/candiedyaml"
	"os"
)

type Trash struct {
	Package string   `yaml:"package,omitempty"`
	Imports []Import `yaml:"import,omitempty"`
}

type Import struct {
	Package string `yaml:"package,omitempty"`
	Version string `yaml:"version,omitempty"`
	Repo    string `yaml:"repo,omitempty"`
}

func Parse(path string) (*Trash, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	trash := &Trash{}
	if err := yaml.NewDecoder(file).Decode(trash); err != nil {
		return nil, err
	}
	return trash, nil
}
