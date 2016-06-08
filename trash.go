package main

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/rancher/trash/conf"
	"github.com/rancher/trash/util"
)

var Version string = "v0.3.0-dev"

func exit(err error) {
	if err != nil {
		logrus.Fatal(err)
	}
}

func main() {
	app := cli.NewApp()
	app.Version = Version
	app.Author = "@imikushin, @ibuildthecloud"
	app.Usage = "Vendor imported packages and throw away the trash!"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "file, f",
			Value: "trash.conf",
			Usage: "Vendored packages list",
		},
		cli.StringFlag{
			Name:  "directory, C",
			Value: ".",
			Usage: "The directory in which to run, --file is relative to this",
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
			Name:  "debug, d",
			Usage: "Debug logging",
		},
		cli.StringFlag{
			Name:   "cache",
			Usage:  "Cache directory",
			Value:  path.Join(os.Getenv("HOME"), ".trash-cache"),
			EnvVar: "TRASH_CACHE",
		},
	}
	app.Action = func(c *cli.Context) {
		exit(run(c))
	}

	exit(app.Run(os.Args))
}

func run(c *cli.Context) error {
	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	dir := c.String("directory")
	trashFile := c.String("file")
	keep := c.Bool("keep")
	update := c.Bool("update")
	trashDir := c.String("cache")

	if err := os.Chdir(dir); err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	logrus.Debugf("dir: '%s'", dir)

	for _, trashFile = range []string{trashFile, "trash.yml", "glide.yaml", "glide.yml", "trash.yaml"} {
		if _, err = os.Stat(trashFile); err == nil {
			break
		}
	}
	if err != nil {
		if os.IsNotExist(err) && update {
			trashFile = c.String("file")
			logrus.Warnf("Trash! '%s' not found, creating a new one!", trashFile)
			if _, err = os.Create(trashFile); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	logrus.Infof("Trash! Reading file: '%s'", trashFile)

	trashConf, err := conf.Parse(trashFile)
	if err != nil {
		return err
	}
	if update {
		return updateTrash(trashDir, dir, trashFile, trashConf)
	}
	if err := vendor(keep, trashDir, dir, trashConf); err != nil {
		return err
	}
	if keep {
		return nil
	}
	return cleanup(dir, trashConf)
}

func updateTrash(trashDir, dir, trashFile string, trashConf *conf.Trash) error {
	// TODO collect imports, create `trashConf *conf.Trash`
	rootPackage := trashConf.Package
	if rootPackage == "" {
		rootPackage = guessRootPackage(dir)
	}

	os.MkdirAll(filepath.Join(trashDir, "src"), 0755)
	os.Setenv("GOPATH", trashDir)

	importMap := map[string]conf.Import{}
	for _, i := range trashConf.Imports {
		importMap[i.Package] = i
	}

	var imports util.Packages
	importsLen := 0
	libRoot := filepath.Join(trashDir, "src")

	for imports = collectImports(rootPackage, libRoot); len(imports) > importsLen; imports = collectImports(rootPackage, libRoot) {

		importsLen = len(imports)

		for pkg := range imports {
			i, ok := trashConf.Get(pkg)
			if !ok {
				i = conf.Import{Package: pkg}
			}
			i.Version = "master"
			if pkgComponents := strings.Split(pkg, "/"); !strings.Contains(pkgComponents[0], ".") {
				continue
			}
			if pkg == rootPackage || strings.HasPrefix(pkg, rootPackage+"/") {
				continue
			}
			prepareCache(trashDir, i)
			checkout(trashDir, i)
		}
	}

	trashConf = &conf.Trash{Package: rootPackage}
	for pkg := range imports {
		if pkgComponents := strings.Split(pkg, "/"); !strings.Contains(pkgComponents[0], ".") {
			continue
		}
		if pkg == rootPackage || strings.HasPrefix(pkg, rootPackage+"/") {
			continue
		}
		i, ok := trashConf.Get(pkg)
		if !ok {
			i = conf.Import{Package: pkg}
		}
		i.Version = getLatestVersion(libRoot, pkg)
		trashConf.Imports = append(trashConf.Imports, i)
	}

	os.Chdir(dir)
	//trashConf.Dump(trashFile)

	return nil
}

func getLatestVersion(libRoot, pkg string) string {
	return "master" // TODO impl
}

func vendor(keep bool, trashDir, dir string, trashConf *conf.Trash) error {
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
		prepareCache(trashDir, i)
		checkout(trashDir, i)
	}

	vendorDir := path.Join(dir, "vendor")
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

func prepareCache(trashDir string, i conf.Import) {
	logrus.WithFields(logrus.Fields{"trashDir": trashDir, "i": i}).Debug("entering prepareCache")
	os.Chdir(trashDir)
	repoDir := path.Join(trashDir, "src", i.Package)
	if err := checkGitRepo(trashDir, repoDir, i); err != nil {
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
	if isBranch(remoteName(i.Repo), i.Version) {
		version = remoteName(i.Repo) + "/" + i.Version
	}
	if bytes, err := exec.Command("git", "checkout", "-f", "--detach", version).CombinedOutput(); err != nil {
		logrus.Debugf("Error running `git checkout -f --detach %s`:\n%s", version, bytes)
		if err := fetch(i); err != nil {
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

func checkGitRepo(trashDir, repoDir string, i conf.Import) error {
	logrus.WithFields(logrus.Fields{"repoDir": repoDir, "i": i}).Debug("checkGitRepo")
	if err := os.Chdir(repoDir); err != nil {
		if os.IsNotExist(err) {
			return cloneGitRepo(trashDir, repoDir, i)
		} else {
			logrus.Errorf("repoDir '%s' cannot be CD'ed to", repoDir)
			return err
		}
	}
	if bytes, err := exec.Command("git", "status").CombinedOutput(); err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Warnf("`git status` failed:\n%s", bytes)
		os.Chdir(trashDir)
		return cloneGitRepo(trashDir, repoDir, i)
	}
	if i.Repo != "" && !remoteExists(remoteName(i.Repo)) {
		addRemote(i.Repo)
	} else if !remoteExists("origin") {
		return cloneGitRepo(trashDir, repoDir, i)
	}
	return nil
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

func cloneGitRepo(trashDir, repoDir string, i conf.Import) error {
	logrus.Infof("Preparing cache for '%s'", i.Package)
	os.Chdir(trashDir)
	if err := os.RemoveAll(repoDir); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "repoDir": repoDir}).Error("os.RemoveAll() failed")
		return err
	}
	if bytes, err := exec.Command("go", "get", "-d", "-f", "-u", i.Package).CombinedOutput(); err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Debugf("`go get -d -f -u %s` returned err:\n%s", i.Package, bytes)
	}
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "repoDir": repoDir}).Error("os.MkdirAll() failed")
		return err
	}
	os.Chdir(repoDir)
	if err := exec.Command("git", "status").Run(); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "repoDir": repoDir}).Debug("not a git repo, creating one")
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
					sch <- v.Path.Value[1 : len(v.Path.Value)-1]
					logrus.Debugf("listImports, sch <- '%s'", v.Path.Value[1:len(v.Path.Value)-1])
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

func listPackages(rootPackage string) util.Packages {
	r := util.Packages{}
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logrus.Warning(err)
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if path == "vendor" ||
			strings.HasSuffix(path, "vendor/") ||
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

func collectImports(rootPackage, libRoot string) util.Packages {
	logrus.Infof("Collecting packages in '%s'", rootPackage)

	imports := util.Packages{}
	packages := listPackages(rootPackage)

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

func removeUnusedImports(imports util.Packages) error {
	importsParents := util.Packages{}
	for i := range imports {
		importsParents.Merge(parentPackages("", i))
	}

	return filepath.Walk("vendor", func(path string, info os.FileInfo, err error) error {
		logrus.Debugf("removeUnusedImports, path: '%s', err: '%v'", path, err)
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if path == "vendor" {
			return nil
		}
		if !info.IsDir() {
			pkg := path[len("vendor/"):strings.LastIndex(path, "/")]
			if strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, ".go") && !imports[pkg] {
				logrus.Infof("Removing unused source file: '%s'", path)
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
		pkg := path[len("vendor/"):]
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

func removeEmptyDirs() error {
	for count := 1; count != 0; {
		count = 0
		if err := filepath.Walk("vendor", func(path string, info os.FileInfo, err error) error {
			logrus.Debugf("removeEmptyDirs, path: '%s', err: '%v'", path, err)
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			if err != nil {
				return err
			}
			if path == "vendor" {
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
	logrus.Info("Trying to guess the root package from directory structure")
	srcPath := path.Join(dir, "..", "..", "..", "..", "src")
	if _, err := os.Stat(srcPath); err != nil {
		logrus.Fatalf("It didn't work: '%s' does not exist or something: %s", srcPath, err)
	}
	srcPath = filepath.Clean(srcPath)
	logrus.Debugf("srcPath: '%s'", srcPath)
	return dir[len(srcPath+"/"):]
}

func cleanup(dir string, trashConf *conf.Trash) error {
	rootPackage := trashConf.Package
	if rootPackage == "" {
		rootPackage = guessRootPackage(dir)
	}

	logrus.Debugf("rootPackage: '%s'", rootPackage)

	os.Chdir(dir)

	imports := collectImports(rootPackage, "vendor")
	if err := removeUnusedImports(imports); err != nil {
		logrus.Errorf("Error removing unused dirs: %v", err)
	}
	if err := removeEmptyDirs(); err != nil {
		logrus.Errorf("Error removing empty dirs: %v", err)
	}
	for _, i := range trashConf.Imports {
		if _, err := os.Stat(dir + "/vendor/" + i.Package); err != nil {
			if os.IsNotExist(err) {
				logrus.Warnf("Package '%s' has been completely removed: it's probably useless (in trash.conf)", i.Package)
			} else {
				logrus.Errorf("os.Stat() failed for: %s", dir+"/vendor/"+i.Package)
			}
		}
	}
	return nil
}
