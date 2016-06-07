package conf

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Sirupsen/logrus"
	yaml "github.com/cloudfoundry-incubator/candiedyaml"
)

type Trash struct {
	Package   string   `yaml:"package,omitempty"`
	Imports   []Import `yaml:"import,omitempty"`
	importMap map[string]Import
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
	uniq := make([]Import, 0, len(t.Imports))
	t.importMap = map[string]Import{}
	for _, i := range t.Imports {
		if _, ok := t.importMap[i.Package]; ok {
			logrus.Warnf("Package '%s' has duplicates (in trash.conf)", i.Package)
			continue
		}
		uniq = append(uniq, i)
		t.importMap[i.Package] = i
	}
	t.Imports = uniq
}

func (t *Trash) Get(pkg string) (Import, bool) {
	i, ok := t.importMap[pkg]
	return i, ok
}

func (t *Trash) Dump(path string) error {
	file, err := os.Create(path)
	defer file.Close()
	if err != nil {
		return err
	}

	w := bufio.NewWriter(file)
	defer w.Flush()

	fmt.Fprintln(w, "# trash.conf")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "# package")
	fmt.Fprintln(w, t.Package)
	fmt.Fprintln(w)

	for _, i := range t.Imports {
		fmt.Fprintln(w, i.Package, i.Version, i.Repo)
		fmt.Println(i.Package, i.Version, i.Repo)
	}

	return nil
}
