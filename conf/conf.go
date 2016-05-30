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

	trash = &Trash{}
	_, err = file.Seek(0, 0)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bufio.NewReader(file))
	for scanner.Scan() {
		line := scanner.Text()
		if commentStart := strings.Index(line, "#"); commentStart >= 0 {
			line = line[0:commentStart]
		}
		if line = strings.TrimSpace(line); line == "" {
			continue
		}
		fields := strings.Fields(line)

		if len(fields) == 1 && trash.Package == "" {
			trash.Package = fields[0] // use the first 1-field line as the root package
			logrus.Infof("Using '%s' as the project's root package (from trash.conf)", trash.Package)
			continue
		}

		packageImport := Import{}
		packageImport.Package = fields[0] // at least 1 field at this point: trimmed the line and skipped empty
		if len(fields) > 2 {
			packageImport.Repo = fields[2]
		}
		if len(fields) > 1 {
			packageImport.Version = fields[1]
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
