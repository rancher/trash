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
	Package   string            `yaml:"package,omitempty"`
	Imports   []Import          `yaml:"import,omitempty"`
	Excludes  []string          `yaml:"exclude,omitempty"`
	Packages  []string          `yaml:"packages,omitempty"`
	ImportMap map[string]Import `yaml:"-"`
	confFile  string            `yaml:"-"`
	yamlType  bool              `yaml:"-"`
}

type Import struct {
	Package string `yaml:"package,omitempty"`
	Version string `yaml:"version,omitempty"`
	Repo    string `yaml:"repo,omitempty"`
	Update  bool   `yaml:"-"`
	Options `yaml:",inline"`
}

type Imports []Import

func (i Imports) Len() int {
	return len(i)
}

func (i Imports) Less(k, j int) bool {
	return strings.Compare(i[k].Package, i[j].Package) <= 0
}

func (i Imports) Swap(k, j int) {
	tmp := i[j]
	i[j] = i[k]
	i[k] = tmp
	return
}

type Options struct {
	Transitive bool `yaml:"transitive,omitempty"`
	Staging    bool `yaml:"staging,omitempty"`
}

type ExportMap struct {
	Imports map[string]Import `yaml:"imports,omitempty"`
}

func Parse(path string) (*Conf, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	trashConf := &Conf{confFile: path}
	if yaml.NewDecoder(file).Decode(trashConf) == nil {
		trashConf.yamlType = true
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
		// If we have a `-` suffix, it's an exclude pattern
		if fields[0][0] == '-' {
			trashConf.Excludes = append(trashConf.Excludes, strings.TrimSpace(fields[0][1:]))
			continue
		}

		if strings.HasPrefix(fields[0], "package=") {
			trashConf.Packages = append(trashConf.Packages, strings.TrimPrefix(fields[0], "package="))
			continue
		}

		// Otherwise it's an import pattern
		packageImport := Import{}
		packageImport.Package = fields[0] // at least 1 field at this point: trimmed the line and skipped empty
		if len(fields) > 3 {
			packageImport.Options = parseOptions(fields[3])
		}
		if len(fields) > 2 {
			if strings.Contains(fields[2], "=") {
				packageImport.Options = parseOptions(fields[2])
			} else {
				packageImport.Repo = fields[2]
			}
		}
		if len(fields) > 1 {
			packageImport.Version = fields[1]
		}
		trashConf.Imports = append(trashConf.Imports, packageImport)
	}

	trashConf.Dedupe()
	return trashConf, nil
}

// Other options besides include_transitive can be included in the future
func parseOptions(options string) Options {
	var importOptions Options
	parts := strings.Split(options, ",")
	for _, part := range parts {
		kvParts := strings.Split(part, "=")
		if len(kvParts) > 1 && kvParts[1] == "true" {
			switch kvParts[0] {
			case "transitive":
				importOptions.Transitive = true
			case "staging":
				importOptions.Staging = true
			}
		}
	}
	return importOptions
}

// Dedupe deletes duplicates and sorts the imports
func (t *Conf) Dedupe() {
	t.ImportMap = map[string]Import{}
	for _, i := range t.Imports {
		if _, ok := t.ImportMap[i.Package]; ok {
			logrus.Debugf("Package '%s' has duplicates (in %s)", i.Package, t.confFile)
			continue
		}
		t.ImportMap[i.Package] = i
	}
	ps := make([]string, 0, len(t.ImportMap))
	for p := range t.ImportMap {
		ps = append(ps, p)
	}
	sort.Strings(ps)
	imports := make([]Import, 0, len(t.ImportMap))
	for _, p := range ps {
		imports = append(imports, t.ImportMap[p])
	}
	t.Imports = imports
}

func (t *Conf) Get(pkg string) (Import, bool) {
	i, ok := t.ImportMap[pkg]
	return i, ok
}

func (t *Conf) Dump(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// If a previous version was in yaml format, preserve it
	if t.yamlType {
		return yaml.NewEncoder(file).Encode(t)
	}
	// Otherwise create a flat config file
	w := bufio.NewWriter(file)
	defer w.Flush()

	fmt.Fprintln(w, "# package")
	fmt.Fprintln(w, t.Package)

	if len(t.Imports) > 0 {
		fmt.Fprintln(w, "\n# import")
		for _, i := range t.Imports {
			s := fmt.Sprintf("%s\t%s\t%s", i.Package, i.Version, i.Repo)
			fmt.Fprintln(w, strings.TrimSpace(s))
		}
	}
	if len(t.Excludes) > 0 {
		fmt.Fprintln(w, "\n# exclude")
		for _, pkg := range t.Excludes {
			fmt.Fprintln(w, "-"+strings.TrimSpace(pkg))
		}
	}
	return nil
}

func (t *Conf) ConfFile() string {
	return t.confFile
}
