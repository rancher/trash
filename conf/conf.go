package conf

import (
	"bufio"
	"os"
	"strings"

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
	defer file.Close()

	trash := &Trash{}
	if err := yaml.NewDecoder(file).Decode(trash); err == nil {
		trash.deleteDups()
		return trash, nil
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)
	scanner := bufio.NewScanner(reader)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		var packageImport Import
		if len(fields) > 2 {
			packageImport.Repo = fields[2]
		}
		if len(fields) > 1 {
			packageImport.Version = fields[1]
		}
		if len(fields) > 0 {
			packageImport.Package = fields[0]
		}
		trash.Imports = append(trash.Imports, packageImport)
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
			logrus.Warnf("Package '%s' has duplicates (in trash.conf)", i.Package)
			continue
		}
		uniq = append(uniq, i)
		seen[i.Package] = true
	}
	t.Imports = uniq
}
