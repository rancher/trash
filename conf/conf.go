package conf

import (
	"bufio"
	"fmt"
	"os"
	"sort"
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
		trash.Dedupe()
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

	trash.Dedupe()
	return trash, nil
}

// Dedupe deletes duplicates and sorts the imports
func (t *Trash) Dedupe() {
	t.importMap = map[string]Import{}
	for _, i := range t.Imports {
		if _, ok := t.importMap[i.Package]; ok {
			logrus.Debugf("Package '%s' has duplicates (in trash.conf)", i.Package)
			continue
		}
		t.importMap[i.Package] = i
	}
	ps := make([]string, 0, len(t.importMap))
	for p := range t.importMap {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	imports := make([]Import, 0, len(t.importMap))
	for _, p := range ps {
		imports = append(imports, t.importMap[p])
	}
	t.Imports = imports
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
		s := fmt.Sprintf("%s\t%s\t%s", i.Package, i.Version, i.Repo)
		fmt.Fprintln(w, strings.TrimSpace(s))
	}

	return nil
}
