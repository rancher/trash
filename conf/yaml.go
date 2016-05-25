package conf

import (
	"os"

	"github.com/Sirupsen/logrus"
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
		if seen[i.Package] {
			logrus.Warnf("Package '%s' has duplicates (in trash.yml)", i.Package)
			continue
		}
		uniq = append(uniq, i)
		seen[i.Package] = true
	}
	t.Imports = uniq
}
