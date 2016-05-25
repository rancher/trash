package conf

import (
	"os"

	yaml "github.com/cloudfoundry-incubator/candiedyaml"
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
	trash.deleteDups()
	return trash, nil
}

// deleteDups delete duplicate imports
func (t *Trash) deleteDups() {
	seen := make(map[string]bool)
	uniq := make([]Import, 0, len(t.Imports))
	for _, i := range t.Imports {
		if _, ok := seen[i.Package]; ok {
			continue
		}
		uniq = append(uniq, i)
		seen[i.Package] = true
	}
	t.Imports = uniq
}
