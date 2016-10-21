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

type Conf struct {
	Package   string   `yaml:"package,omitempty"`
	Imports   []Import `yaml:"import,omitempty"`
	importMap map[string]Import
	confFile  string
}

type Import struct {
	Package string `yaml:"package,omitempty"`
	Version string `yaml:"version,omitempty"`
	Repo    string `yaml:"repo,omitempty"`
}

func Parse(path string) (*Conf, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	trashConf := &Conf{confFile: path}
	if err := yaml.NewDecoder(file).Decode(trashConf); err == nil {
		trashConf.Dedupe()
		return trashConf, nil
	}

	trashConf = &Conf{confFile: path}
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

		if len(fields) == 1 && trashConf.Package == "" {
			trashConf.Package = fields[0] // use the first 1-field line as the root package
			logrus.Infof("Using '%s' as the project's root package (from %s)", trashConf.Package, trashConf.confFile)
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
		trashConf.Imports = append(trashConf.Imports, packageImport)
	}

	trashConf.Dedupe()
	return trashConf, nil
}

// Dedupe deletes duplicates and sorts the imports
func (t *Conf) Dedupe() {
	t.importMap = map[string]Import{}
	for _, i := range t.Imports {
		if _, ok := t.importMap[i.Package]; ok {
			logrus.Debugf("Package '%s' has duplicates (in %s)", i.Package, t.confFile)
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

func (t *Conf) Get(pkg string) (Import, bool) {
	i, ok := t.importMap[pkg]
	return i, ok
}

func (t *Conf) Dump(path string, useYaml bool) error {
	file, err := os.Create(path)
	defer file.Close()
	if err != nil {
		return err
	}

	w := bufio.NewWriter(file)
	defer w.Flush()

	if useYaml {
		encoder := yaml.NewEncoder(file)
		err = encoder.Encode(t)
		if err != nil {
			println("Failed to encode document:", err.Error())
			return err
		}
	} else {
		fmt.Fprintln(w, "# package")
		fmt.Fprintln(w, t.Package)
		fmt.Fprintln(w)

		for _, i := range t.Imports {
			s := fmt.Sprintf("%s\t%s\t%s", i.Package, i.Version, i.Repo)
			fmt.Fprintln(w, strings.TrimSpace(s))
		}
	}

	return nil
}

func (t *Conf) ConfFile() string {
	return t.confFile
}
