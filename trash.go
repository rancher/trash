package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"

	"github.com/rancher/trash/conf"
	"github.com/rancher/trash/util"
)

var Version string = "v0.3.0-dev"

func main() {
	app := cli.NewApp()
	app.Version = Version
	app.Author = "@imikushin, @ibuildthecloud"
	app.Usage = "Vendor imported packages and throw away the trash!"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "file, f",
			Value: "vendor.conf",
			Usage: "Vendored packages list",
		},
		cli.StringFlag{
			Name:  "directory, C",
			Value: ".",
			Usage: "The directory in which to run, --file is relative to this",
		},
		cli.StringFlag{
			Name:  "target, T",
			Value: "vendor",
			Usage: "The directory to store results",
		},
		cli.BoolFlag{
			Name:  "keep, k",
			Usage: "Keep all downloaded vendor code (preserving .git dirs)",
		},
		cli.BoolFlag{
			Name:  "update, u",
			Usage: "Update vendored packages, add missing ones",
		},
		cli.BoolFlag{
			Name:  "insecure",
			Usage: "Pass -insecure to 'go get'",
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "Debug logging",
		},
		cli.StringFlag{
			Name:   "cache",
			Usage:  "Cache directory",
			Value:  path.Join(os.Getenv("HOME"), ".trash-cache"),
			EnvVar: "TRASH_CACHE",
		},
		cli.StringFlag{
			Name:   "gopath",
			Hidden: true,
			EnvVar: "GOPATH",
		},
	}
	app.Action = run

	app.Run(os.Args)
}

var gopath string

func run(c *cli.Context) error {
	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	dir := c.String("directory")
	targetDir := c.String("target")
	confFile := c.String("file")
	keep := c.Bool("keep")
	update := c.Bool("update")
	insecure := c.Bool("insecure")
	trashDir := c.String("cache")
	gopath = c.String("gopath")

	trashDir, err := filepath.Abs(trashDir)
	if err != nil {
		return err
	}

	if err := os.Chdir(dir); err != nil {
		return err
	}
	dir, err = os.Getwd()
	if err != nil {
		return err
	}
	logrus.Debugf("dir: '%s'", dir)

	for _, confFile = range []string{confFile, "trash.conf", "vndr.cfg", "vendor.manifest", "trash.yml", "glide.yaml", "glide.yml", "trash.yaml"} {
		if _, err = os.Stat(confFile); err == nil {
			break
		}
	}
	if err != nil {
		if os.IsNotExist(err) && update {
			confFile = c.String("file")
			logrus.Warnf("Trash! '%s' not found, creating a new one!", confFile)
			if _, err = os.Create(confFile); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	logrus.Infof("Trash! Reading file: '%s'", confFile)

	trashConf, err := conf.Parse(confFile)
	if err != nil {
		return err
	}
	if update {
		return updateTrash(trashDir, dir, targetDir, confFile, trashConf, insecure)
	}
	if err := vendor(keep, trashDir, dir, targetDir, trashConf, insecure); err != nil {
		return err
	}
	if keep {
		return nil
	}
	return cleanup(dir, targetDir, trashConf)
}

func updateTrash(trashDir, dir, targetDir, trashFile string, trashConf *conf.Conf, insecure bool) error {
	// TODO collect imports, create `trashConf *conf.Trash`
	rootPackage := trashConf.Package
	if rootPackage == "" {
		rootPackage = guessRootPackage(dir)
	}

	os.MkdirAll(filepath.Join(trashDir, "src"), 0755)
	os.Setenv("GOPATH", trashDir)

	libRoot := filepath.Join(trashDir, "src")
	importsLen := 0

	os.Chdir(dir)
	imports := collectImports(rootPackage, libRoot, targetDir)
	for len(imports) > importsLen {
		importsLen = len(imports)
		for pkg := range imports {
			i, ok := trashConf.Get(pkg)
			if !ok {
				i = conf.Import{Package: pkg}
			}
			i.Version = "master"
			if pkg == rootPackage || strings.HasPrefix(pkg, rootPackage+"/") {
				continue
			}
			prepareCache(trashDir, i, insecure)
			checkout(trashDir, i)
		}
		os.Chdir(dir)
		imports = collectImports(rootPackage, libRoot, targetDir)
	}

	trashConf.Package = rootPackage // Overwrite possibly non existent root package name
	trashConf.Imports = nil         // Drop any old imports to include only new ones
	for pkg := range imports {
		if pkg == rootPackage || strings.HasPrefix(pkg, rootPackage+"/") {
			continue
		}
		pkg, err := topLevel(pkg, libRoot)
		if err != nil {
			return err
		}
		i, ok := trashConf.Get(pkg) // Get uses importMap for meta fields, which was preserved above
		if !ok {
			i = conf.Import{Package: pkg}
		}
		i.Version, err = getLatestVersion(libRoot, pkg)
		if err != nil {
			return err
		}
		os.Chdir(dir)
		trashConf.Imports = append(trashConf.Imports, i)
	}
	trashConf.Dedupe()

	os.Chdir(dir)
	trashConf.Dump(trashFile)

	return nil
}

func topLevel(pkg, libRoot string) (string, error) {
	if err := os.Chdir(filepath.Join(libRoot, pkg)); err != nil {
		return "", err
	}
	bytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(bytes))
	return s[len(libRoot)+1:], nil
}

func getLatestVersion(libRoot, pkg string) (string, error) {
	if err := os.Chdir(filepath.Join(libRoot, pkg)); err != nil {
		return "", err
	}
	bytes, err := exec.Command("git", "describe", "--tags", "--always").Output()
	if err != nil {
		return "", err
	}
	s := strings.TrimSpace(string(bytes))
	return s, nil
}

func vendor(keep bool, trashDir, dir, targetDir string, trashConf *conf.Conf, insecure bool) error {
	logrus.WithFields(logrus.Fields{"keep": keep, "dir": dir, "trashConf": trashConf}).Debug("vendor")
	defer os.Chdir(dir)

	for _, i := range trashConf.Imports {
		if i.Version == "" {
			return fmt.Errorf("version not specified for package '%s'", i.Package)
		}
	}

	os.MkdirAll(trashDir, 0755)
	os.Setenv("GOPATH", trashDir)

	for _, i := range trashConf.Imports {
		prepareCache(trashDir, i, insecure)
		checkout(trashDir, i)
	}

	vendorDir := path.Join(dir, targetDir)
	os.RemoveAll(vendorDir)
	os.MkdirAll(vendorDir, 0755)

	logrus.Info("Copying deps...")
	for _, i := range trashConf.Imports {
		cpy(vendorDir, trashDir, i)
	}
	logrus.Info("Copying deps... Done")
	if !keep {
		if err := filepath.Walk(vendorDir, func(path string, info os.FileInfo, err error) error {
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return nil
			}
			if _, d := filepath.Split(path); d == ".git" {
				logrus.Infof("removing '%s", path)
				return os.RemoveAll(path)
			}
			return nil
		}); err != nil {
			logrus.Errorf("Error stripping .git dirs: %s", err)
			return err
		}
	}

	return nil
}

func prepareCache(trashDir string, i conf.Import, insecure bool) {
	logrus.WithFields(logrus.Fields{"trashDir": trashDir, "i": i}).Debug("entering prepareCache")
	os.Chdir(trashDir)
	repoDir := path.Join(trashDir, "src", i.Package)
	if err := checkGitRepo(trashDir, repoDir, i, insecure); err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Fatal("checkGitRepo failed")
	}
}

func isBranch(remote, version string) bool {
	b := remote + "/" + version
	logrus.Debugf("Checking if '%s' is a branch", b)
	for l := range util.CmdOutLines(exec.Command("git", "branch", "--list", "-r", b)) {
		if strings.TrimSpace(l) == b {
			return true
		}
	}
	return false
}

func checkout(trashDir string, i conf.Import) {
	logrus.WithFields(logrus.Fields{"trashDir": trashDir, "i": i}).Debug("entering checkout")
	repoDir := path.Join(trashDir, "src", i.Package)
	if err := os.Chdir(repoDir); err != nil {
		logrus.Fatalf("Could not change to dir '%s'", repoDir)
	}
	logrus.Infof("Checking out '%s', commit: '%s'", i.Package, i.Version)
	version := i.Version
	if i.Version == "master" || isBranch(remoteName(i.Repo), i.Version) {
		version = remoteName(i.Repo) + "/" + i.Version
		if err := fetch(i); err != nil {
			logrus.WithFields(logrus.Fields{"i": i}).Fatalf("fetch failed")
		}
	}
	if bytes, err := exec.Command("git", "checkout", "-f", "--detach", version).CombinedOutput(); err != nil {
		logrus.Debugf("Error running `git checkout -f --detach %s`:\n%s", version, bytes)
		if i.Version == "master" {
			logrus.Warn("Failed to checkout 'master' branch: checking out the latest commit git can find")
			bytes, err := exec.Command("git", "log", "--all", "--pretty=oneline", "--abbrev-commit", "-1").Output()
			if err != nil {
				logrus.Fatalf("Failed to get latest commit with `git log --all --pretty=oneline --abbrev-commit -1`: %s", err)
			}
			version = strings.Fields(strings.TrimSpace(string(bytes)))[0]
		} else if err := fetch(i); err != nil {
			logrus.WithFields(logrus.Fields{"i": i}).Fatalf("fetch failed")
		}
		logrus.Debugf("Retrying!: `git checkout -f --detach %s`", version)
		if bytes, err := exec.Command("git", "checkout", "-f", "--detach", version).CombinedOutput(); err != nil {
			logrus.Fatalf("`git checkout -f --detach %s` failed:\n%s", version, bytes)
		}
	}
}

func cpy(vendorDir, trashDir string, i conf.Import) {
	repoDir := path.Join(trashDir, "src", i.Package)
	target, _ := path.Split(path.Join(vendorDir, i.Package))
	os.MkdirAll(target, 0755)
	if bytes, err := exec.Command("cp", "-a", repoDir, target).CombinedOutput(); err != nil {
		logrus.Fatalf("`cp -a %s %s` failed:\n%s", repoDir, target, bytes)
	}
}

func checkGitRepo(trashDir, repoDir string, i conf.Import, insecure bool) error {
	logrus.WithFields(logrus.Fields{"repoDir": repoDir, "i": i}).Debug("checkGitRepo")
	if err := os.Chdir(repoDir); err != nil {
		if os.IsNotExist(err) {
			return cloneGitRepo(trashDir, repoDir, i, insecure)
		} else {
			logrus.Errorf("repoDir '%s' cannot be CD'ed to", repoDir)
			return err
		}
	}
	if !isCurrentDirARepo(trashDir) {
		os.Chdir(trashDir)
		return cloneGitRepo(trashDir, repoDir, i, insecure)
	}
	if i.Repo != "" && !remoteExists(remoteName(i.Repo)) {
		addRemote(i.Repo)
	} else if !remoteExists("origin") {
		return cloneGitRepo(trashDir, repoDir, i, insecure)
	}
	return nil
}

func isCurrentDirARepo(trashDir string) bool {
	d, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Error getting current dir: %s", err)
	}
	bytes, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		logrus.Debugf("Not in a git repo: `git rev-parse --show-toplevel` in dir %s failed: %s", d, err)
		return false
	}
	return strings.HasPrefix(string(bytes), trashDir+"/src/")
}

func remoteExists(remoteName string) bool {
	lines := util.CmdOutLines(exec.Command("git", "remote"))
	for line := range lines {
		if strings.TrimSpace(line) == remoteName {
			return true
		}
	}
	return false
}

func addRemote(url string) {
	remoteName := remoteName(url)
	if bytes, err := exec.Command("git", "remote", "add", "-f", remoteName, url).CombinedOutput(); err != nil {
		logrus.Debugf("err: '%v', out: '%s'", err, string(bytes))
		if strings.Contains(string(bytes), fmt.Sprintf("remote %s already exists", remoteName)) {
			logrus.Warnf("Already have the remote '%s', '%s'", remoteName, url)
		} else {
			logrus.Errorf("Could not add remote '%s' '%s'", remoteName, url)
		}
	}
}

func remoteName(url string) string {
	if url == "" {
		return "origin"
	}
	ss := sha1.Sum([]byte(url))
	return hex.EncodeToString(ss[:])[:7]
}

func cloneGitRepo(trashDir, repoDir string, i conf.Import, insecure bool) error {
	logrus.Infof("Preparing cache for '%s'", i.Package)
	os.Chdir(trashDir)
	if err := os.RemoveAll(repoDir); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "repoDir": repoDir}).Error("os.RemoveAll() failed")
		return err
	}
	args := []string{"get", "-d", "-f", "-u"}
	if insecure {
		args = append(args, "-insecure")
	}
	args = append(args, i.Package)
	if bytes, err := exec.Command("go", args...).CombinedOutput(); err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Debugf("`go %s` returned err:\n%s", strings.Join(args, " "), bytes)
	}
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "repoDir": repoDir}).Error("os.MkdirAll() failed")
		return err
	}
	os.Chdir(repoDir)
	if !isCurrentDirARepo(trashDir) {
		logrus.WithFields(logrus.Fields{"repoDir": repoDir}).Debug("not a git repo, creating one")
		exec.Command("git", "init", "-q").Run()
	}
	if i.Repo != "" {
		addRemote(i.Repo)
	}
	return nil
}

func fetch(i conf.Import) error {
	remote := remoteName(i.Repo)
	logrus.Infof("Fetching latest commits from '%s' for '%s'", remote, i.Package)
	if bytes, err := exec.Command("git", "fetch", "-f", "-t", remote).CombinedOutput(); err != nil {
		logrus.Errorf("`git fetch -f -t %s` failed:\n%s", remote, bytes)
		return err
	}
	return nil
}

func parentPackages(root, p string) util.Packages {
	r := util.Packages{}
	lenRoot := len(root)
	for len(p) > lenRoot {
		r[p] = true
		p, _ = path.Split(p)
		if len(p) > 0 && p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
	}
	return r
}

func listImports(rootPackage, libRoot, pkg string) <-chan util.Packages {
	pkgPath := "."
	if pkg != rootPackage {
		if strings.HasPrefix(pkg, rootPackage+"/") {
			pkgPath = pkg[len(rootPackage)+1:]
		} else {
			pkgPath = libRoot + "/" + pkg
		}
	}
	logrus.Debugf("listImports, pkgPath: '%s'", pkgPath)
	sch := make(chan string)
	noVendoredTests := func(info os.FileInfo) bool {
		if strings.HasPrefix(pkgPath, libRoot+"/") && strings.HasSuffix(info.Name(), "_test.go") {
			return false
		}
		return true
	}
	go func() {
		defer close(sch)

		// Gather all the Go imports
		ps, err := parser.ParseDir(token.NewFileSet(), pkgPath, noVendoredTests, parser.ImportsOnly)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Debugf("listImports, pkgPath does not exist: %s", err)
			} else {
				logrus.Errorf("Error parsing imports, pkgPath: '%s', err: '%s'", pkgPath, err)
			}
			return
		}
		logrus.Infof("Collecting imports for package '%s'", pkg)
		for _, p := range ps {
			for _, f := range p.Files {
				for _, v := range f.Imports {
					imp := v.Path.Value[1 : len(v.Path.Value)-1]
					if pkgComponents := strings.Split(imp, "/"); !strings.Contains(pkgComponents[0], ".") {
						continue
					} else if pkgComponents[0] == "." || pkgComponents[0] == ".." {
						imp = filepath.Clean(filepath.Join(pkg, imp))
					}
					if imp == rootPackage || strings.HasPrefix(imp, rootPackage+"/") {
						continue
					}
					sch <- imp
					logrus.Debugf("listImports, sch <- '%s'", v.Path.Value[1:len(v.Path.Value)-1])
				}
			}
		}
		// Gather all the CGO imports
		ps, err = parser.ParseDir(token.NewFileSet(), pkgPath, noVendoredTests, parser.ParseComments)
		if err != nil {
			if os.IsNotExist(err) {
				logrus.Debugf("listImports, pkgPath does not exist: %s", err)
			} else {
				logrus.Errorf("Error parsing comments, pkgPath: '%s', err: '%s'", pkgPath, err)
			}
			return
		}
		logrus.Infof("Collecting CGO imports for package '%s'", pkg)
		for _, p := range ps {
			for _, f := range p.Files {
				// Drill down to locate C preable definitions
				for _, decl := range f.Decls {
					d, ok := decl.(*ast.GenDecl)
					if !ok {
						continue
					}
					for _, spec := range d.Specs {
						s, ok := spec.(*ast.ImportSpec)
						if !ok || s.Path.Value != `"C"` {
							continue
						}
						cg := s.Doc
						if cg == nil && len(d.Specs) == 1 {
							cg = d.Doc
						}
						if cg != nil {
							// Extract any includes from the preamble
							for _, line := range strings.Split(cg.Text(), "\n") {
								if line = strings.TrimSpace(line); strings.HasPrefix(line, "#include \"") {
									if includePath := filepath.Dir(line[10 : len(line)-1]); includePath != "." {
										if _, err := os.Stat(filepath.Join(pkgPath, includePath)); !os.IsNotExist(err) {
											sch <- filepath.Clean(filepath.Join(pkg, includePath))
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}()
	lnc := util.MergeStrChans(sch, util.OneStr(pkg))
	return chanPackagesFromLines(lnc)
}

func chanPackagesFromLines(lnc <-chan string) <-chan util.Packages {
	return util.ChanPackages(func() util.Packages {
		r := util.Packages{}
		for v := range lnc {
			r[v] = true
		}
		return r
	})
}

func listPackages(rootPackage, targetDir string) util.Packages {
	r := util.Packages{}
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Warning(err)
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if path == targetDir ||
			strings.HasSuffix(path, targetDir+"/") ||
			path != "." && strings.HasPrefix(path[strings.LastIndex(path, "/")+1:], ".") {
			return filepath.SkipDir
		}
		logrus.Debugf("path: '%s'", path)
		pkgs, err := parser.ParseDir(token.NewFileSet(), path, nil, parser.PackageClauseOnly)
		if err != nil {
			logrus.Error(err)
			return err
		}
		if len(pkgs) > 0 {
			logrus.Debugf("Adding package: '%s'", path)
			if path == "." {
				r[rootPackage] = true
			} else {
				r[rootPackage+"/"+path] = true
			}
		}
		return nil
	})
	return r
}

func collectImports(rootPackage, libRoot, targetDir string) util.Packages {
	logrus.Infof("Collecting packages in '%s'", rootPackage)

	imports := util.Packages{}
	packages := listPackages(rootPackage, targetDir)

	seenPackages := util.Packages{}
	for len(packages) > 0 {
		cs := []<-chan util.Packages{}
		for p := range packages {
			cs = append(cs, listImports(rootPackage, libRoot, p))
		}
		for ps := range util.MergePackagesChans(cs...) {
			imports.Merge(ps)
		}
		seenPackages.Merge(packages)
		packages = util.Packages{}
		for i := range imports {
			if !seenPackages[i] {
				packages[i] = true
			}
		}
	}

	for p := range imports {
		logrus.Debugf("Keeping: '%s'", p)
	}

	logrus.Debugf("imports len: %v", len(imports))
	return imports
}

func removeUnusedImports(imports util.Packages, targetDir string) error {
	importsParents := util.Packages{}
	for i := range imports {
		importsParents.Merge(parentPackages("", i))
	}
	return filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		logrus.Debugf("removeUnusedImports, path: '%s', err: '%v'", path, err)
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if path == targetDir {
			return nil
		}
		if !info.IsDir() {
			pkg := path[len(targetDir+"/"):strings.LastIndex(path, "/")]
			if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".go") && !imports[pkg] {
				logrus.Debugf("Removing unused source file: '%s'", path)
				if err := os.Remove(path); err != nil {
					if os.IsNotExist(err) {
						return nil
					}
					logrus.Errorf("Error removing file: '%s', err: '%v'", path, err)
					return err
				}
			}
			return nil
		}
		pkg := path[len(targetDir+"/"):]
		if !imports[pkg] && !importsParents[pkg] {
			logrus.Infof("Removing unused dir: '%s'", path)
			err := os.RemoveAll(path)
			if err == nil {
				return filepath.SkipDir
			}
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			logrus.Errorf("Error removing unused dir, path: '%s', err: '%v'", path, err)
			return err
		}
		return nil
	})
}

func removeExcludes(excludes []string, targetDir string) error {
	exclude := make(map[string]bool)
	for _, dir := range excludes {
		exclude[dir] = true
	}
	return filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		logrus.Debugf("removeExcludes, path: '%s', err: '%v'", path, err)
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if path == targetDir {
			return nil
		}
		pkg := path[len(targetDir+"/"):]
		if exclude[pkg] {
			logrus.Infof("Removing excluded dir: '%s'", path)
			err := os.RemoveAll(path)
			if err == nil {
				return filepath.SkipDir
			}
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			logrus.Errorf("Error removing excluded dir, path: '%s', err: '%v'", path, err)
			return err
		}
		return nil
	})
}

func removeEmptyDirs(targetDir string) error {
	for count := 1; count != 0; {
		count = 0
		if err := filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
			logrus.Debugf("removeEmptyDirs, path: '%s', err: '%v'", path, err)
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			if err != nil {
				return err
			}
			if path == targetDir {
				return nil
			}
			if info.IsDir() {
				err := os.Remove(path)
				if err == nil {
					logrus.Infof("Removed Empty dir: '%s'", path)
					count++
					return filepath.SkipDir
				}
				if os.IsNotExist(err) {
					return filepath.SkipDir
				}
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func guessRootPackage(dir string) string {
	logrus.Warn("Trying to guess the root package using GOPATH. It's best to specify it in `vendor.conf`")
	logrus.Warnf("GOPATH is '%s'", gopath)
	if gopath == "" || strings.Contains(gopath, ":") {
		logrus.Fatalf("GOPATH not set or is not a single path. You need to specify the root package!")
	}
	srcPath := filepath.Clean(path.Join(gopath, "src"))
	if !strings.HasPrefix(dir, srcPath+"/") {
		logrus.Fatalf("Your project dir is not a subdir of $GOPATH/src. You need to specify the root package!")
	}
	if _, err := os.Stat(srcPath); err != nil {
		logrus.Fatalf("It didn't work: $GOPATH/src does not exist or something: %s", err)
	}
	logrus.Debugf("srcPath: '%s'", srcPath)
	return dir[len(srcPath+"/"):]
}

func cleanup(dir, targetDir string, trashConf *conf.Conf) error {
	rootPackage := trashConf.Package
	if rootPackage == "" {
		rootPackage = guessRootPackage(dir)
	}

	logrus.Debugf("rootPackage: '%s'", rootPackage)

	os.Chdir(dir)

	imports := collectImports(rootPackage, targetDir, targetDir)
	if err := removeExcludes(trashConf.Excludes, targetDir); err != nil {
		logrus.Errorf("Error removing excluded dirs: %v", err)
	}
	if err := removeUnusedImports(imports, targetDir); err != nil {
		logrus.Errorf("Error removing unused dirs: %v", err)
	}
	if err := removeEmptyDirs(targetDir); err != nil {
		logrus.Errorf("Error removing empty dirs: %v", err)
	}
	for _, i := range trashConf.Imports {
		pth := dir + "/" + targetDir + "/" + i.Package
		if _, err := os.Stat(pth); err != nil {
			if os.IsNotExist(err) {
				logrus.Warnf("Package '%s' has been completely removed: it's probably useless (in %s)", i.Package, trashConf.ConfFile())
			} else {
				logrus.Errorf("os.Stat() failed for: %s", pth)
			}
		}
	}
	return nil
}
