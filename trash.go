package main

import (
	"fmt"
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

var Version string = "0.1.0"

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
			Value: "trash.yml",
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

var possibleTrashFiles = []string{"glide.yaml", "glide.yml", "trash.yaml"}

func run(c *cli.Context) error {
	if c.Bool("debug") {
		logrus.SetLevel(logrus.DebugLevel)
	}

	dir := c.String("directory")
	trashFile := c.String("file")
	keep := c.Bool("keep")

	if err := os.Chdir(dir); err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	logrus.Debugf("dir: '%s'", dir)

	_, err = os.Stat(trashFile)
	for i := 0; os.IsNotExist(err) && i < len(possibleTrashFiles); i++ {
		trashFile = possibleTrashFiles[i]
		_, err = os.Stat(trashFile)
	}
	if os.IsNotExist(err) {
		return err
	}
	logrus.Infof("Trash! Reading file: '%s'", trashFile)

	os.Setenv("GO15VENDOREXPERIMENT", "1")
	if err := vendor(keep, c.String("cache"), dir, trashFile); err != nil {
		return err
	}
	if keep {
		return nil
	}
	return cleanup(dir)
}

func vendor(keep bool, trashDir, dir, trashFile string) error {
	logrus.WithFields(logrus.Fields{"dir": dir, "trashFile": trashFile}).Debug("vendor")
	defer os.Chdir(dir)

	trashConf, err := conf.Parse(trashFile)
	if err != nil {
		return err
	}
	for _, i := range trashConf.Imports {
		if i.Version == "" {
			return fmt.Errorf("version not specified for package '%s'", i.Package)
		}
	}

	os.MkdirAll(trashDir, 0755)
	os.Setenv("GOPATH", trashDir)

	for _, i := range trashConf.Imports {
		prepareCache(trashDir, i)
	}

	for _, i := range trashConf.Imports {
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
			logrus.Error("Error stripping .git dirs: %s", err)
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

func checkout(trashDir string, i conf.Import) {
	logrus.WithFields(logrus.Fields{"trashDir": trashDir, "i": i}).Debug("entering checkout")
	repoDir := path.Join(trashDir, "src", i.Package)
	if err := os.Chdir(repoDir); err != nil {
		logrus.Fatalf("Could not change to dir '%s'", repoDir)
	}
	logrus.Infof("Checking out '%s', commit: '%s'", i.Package, i.Version)
	if bytes, err := exec.Command("git", "checkout", "-f", "--detach", i.Version).CombinedOutput(); err != nil {
		logrus.Debugf("Error running `git checkout -f --detach %s`:\n%s", i.Version, bytes)
		if err := fetch(i); err != nil {
			logrus.WithFields(logrus.Fields{"i": i}).Fatalf("fetch failed")
		}
		logrus.Debugf("Retrying!: `git checkout -f --detach %s`", i.Version)
		if bytes, err := exec.Command("git", "checkout", "-f", "--detach", i.Version).CombinedOutput(); err != nil {
			logrus.Fatalf("`git checkout -f --detach %s` failed:\n%s", i.Version, bytes)
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
	return nil
}

func cloneGitRepo(trashDir, repoDir string, i conf.Import) error {
	logrus.Infof("Preparing cache for '%s'", i.Package)
	os.Chdir(trashDir)
	if err := os.RemoveAll(repoDir); err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "repoDir": repoDir}).Error("os.RemoveAll() failed")
		return err
	}
	if i.Repo != "" {
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			logrus.WithFields(logrus.Fields{"err": err, "repoDir": repoDir}).Error("os.MkdirAll() failed")
			return err
		}
		if bytes, err := exec.Command("git", "clone", i.Repo, repoDir).CombinedOutput(); err != nil {
			logrus.WithFields(logrus.Fields{"err": err}).Errorf("`git clone %s %s` failed:\n%s", i.Repo, repoDir, bytes)
			return err
		}
	} else {
		if bytes, err := exec.Command("go", "get", "-d", "-f", "-u", i.Package).CombinedOutput(); err != nil {
			logrus.WithFields(logrus.Fields{"err": err}).Debugf("`go get -d -f -u %s` returned err:\n%s", i.Package, bytes)
		}
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			logrus.WithFields(logrus.Fields{"err": err, "repoDir": repoDir}).Error("os.MkdirAll() failed")
			return err
		}
	}
	return nil
}

func fetch(i conf.Import) error {
	repo := "origin"
	if i.Repo != "" {
		repo = i.Repo
	}
	logrus.Infof("Fetching latest commits from '%s' for '%s'", repo, i.Package)
	if bytes, err := exec.Command("git", "fetch", "-f", "-t", repo).CombinedOutput(); err != nil {
		logrus.Errorf("`git fetch -f -t %s` failed:\n%s", repo, bytes)
		return err
	}
	return nil
}

// see https://golang.org/doc/install/source (look for "$GOOS and $GOARCH")
var goOsArch [][]string = [][]string{
	{"darwin", "386"},
	{"darwin", "amd64"},
	{"darwin", "arm"},
	{"darwin", "arm64"},
	{"dragonfly", "amd64"},
	{"freebsd", "386"},
	{"freebsd", "amd64"},
	{"freebsd", "arm"},
	{"linux", "386"},
	{"linux", "amd64"},
	{"linux", "arm"},
	{"linux", "arm64"},
	{"linux", "ppc64"},
	{"linux", "ppc64le"},
	{"linux", "mips64"},
	{"linux", "mips64le"},
	{"netbsd", "386"},
	{"netbsd", "amd64"},
	{"netbsd", "arm"},
	{"openbsd", "386"},
	{"openbsd", "amd64"},
	{"openbsd", "arm"},
	{"plan9", "386"},
	{"plan9", "amd64"},
	{"solaris", "amd64"},
	{"windows", "386"},
	{"windows", "amd64"},
}

func parentPackages(rootPackage, p string) util.Packages {
	r := util.Packages{}
	lenRoot := len(rootPackage + "/vendor")
	for len(p) > lenRoot {
		r[p] = true
		p, _ = path.Split(p)
		if len(p) > 0 && p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
	}
	return r
}

func listImports(rootPackage, p string) <-chan util.Packages {
	lnc := util.MergeStrChans(util.CmdOutLines(exec.Command("go", "list", "-f", `{{join .Deps "\n"}}`, p)), util.OneStr(p))
	return util.ChanPackages(func() util.Packages {
		imports := util.Packages{}
		for v := range lnc {
			vendorDirLastIndex := strings.LastIndex(v, "/vendor/")
			if vendorDirLastIndex != -1 {
				v = rootPackage + v[vendorDirLastIndex:]
				imports.Merge(parentPackages(rootPackage, v))
			}
		}
		return imports
	})
}

func listTestImports(rootPackage, p string) <-chan util.Packages {
	lnc := util.CmdOutLines(exec.Command("go", "list", "-f", `{{join .TestImports "\n"}}`, p))
	return util.ChanPackages(func() util.Packages {
		r := util.Packages{}
		for v := range lnc {
			vendorDirLastIndex := strings.LastIndex(v, "/vendor/")
			if vendorDirLastIndex != -1 {
				r[rootPackage+v[vendorDirLastIndex:]] = true
			}
		}
		return r
	})
}

func listPackages(rootPackage string) <-chan util.Packages {
	lnc := util.CmdOutLines(exec.Command("go", "list", rootPackage+"/..."))
	return util.ChanPackages(func() util.Packages {
		r := util.Packages{}
		for v := range lnc {
			if strings.Index(v, "/vendor/") == -1 {
				logrus.Debugf("Adding package: '%s'", v)
				r[v] = true
			}
		}
		return r
	})
}

func collectImports(rootPackage string) util.Packages {
	logrus.Infof("Collecting packages in '%s'", rootPackage)

	imports := util.Packages{}
	packages := util.Packages{}
	testImports := util.Packages{}

	cs := make([]<-chan util.Packages, len(goOsArch))
	for i, t := range goOsArch {
		goOs, goArch := t[0], t[1]
		os.Setenv("GOOS", goOs)
		os.Setenv("GOARCH", goArch)
		cs[i] = listPackages(rootPackage)
	}
	for p := range util.MergePackagesChans(cs...) {
		packages.Merge(p)
	}

	cs = make([]<-chan util.Packages, len(packages)*len(goOsArch))
	i := 0
	for p := range packages {
		logrus.Infof("Collecting test imports of '%s'", p)
		for _, t := range goOsArch {
			goOs, goArch := t[0], t[1]
			os.Setenv("GOOS", goOs)
			os.Setenv("GOARCH", goArch)
			cs[i] = listTestImports(rootPackage, p)
			i++
		}
	}
	for p := range util.MergePackagesChans(cs...) {
		testImports.Merge(p)
	}

	packages.Merge(testImports)

	cs = make([]<-chan util.Packages, len(packages)*len(goOsArch))
	i = 0
	for p := range packages {
		logrus.Infof("Collecting imports for package '%s'", p)
		for _, t := range goOsArch {
			goOs, goArch := t[0], t[1]
			os.Setenv("GOOS", goOs)
			os.Setenv("GOARCH", goArch)
			cs[i] = listImports(rootPackage, p)
			i++
		}
	}
	for p := range util.MergePackagesChans(cs...) {
		imports.Merge(p)
	}

	imports[rootPackage+"/vendor"] = true

	for p := range imports {
		logrus.Infof("Keeping: '%s'", p)
	}

	return imports
}

func removeUnusedImports(rootPackage string, imports util.Packages) error {
	return filepath.Walk(rootPackage+"/vendor", func(path string, info os.FileInfo, err error) error {
		logrus.Debugf("removeUnusedImports, path: '%s', err: '%v'", path, err)
		if os.IsNotExist(err) {
			return filepath.SkipDir
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if !imports[path] {
			logrus.Infof("Removing Unused dir: '%s'", path)
			err := os.RemoveAll(path)
			if err == nil {
				return filepath.SkipDir
			}
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			logrus.Errorf("Error removing Unused dir, path: '%s', err: '%v'", path, err)
			return err
		}
		return nil
	})
}

func removeEmptyDirs(rootPackage string) error {
	for count := 1; count != 0; {
		count = 0
		if err := filepath.Walk(rootPackage+"/vendor", func(path string, info os.FileInfo, err error) error {
			logrus.Debugf("removeEmptyDirs, path: '%s', err: '%v'", path, err)
			if os.IsNotExist(err) {
				return filepath.SkipDir
			}
			if err != nil {
				return err
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

func cleanup(dir string) error {
	gopath := path.Join(dir, "..", "..", "..", "..")
	gopath = filepath.Clean(gopath)
	os.Setenv("GOPATH", gopath)
	logrus.Debugf("gopath: '%s'", gopath)

	rootPackage := dir[len(gopath+"/src/"):]
	logrus.Debugf("rootPackage: '%s'", rootPackage)

	os.Chdir(path.Join(gopath, "src"))

	importsLen := 0
	for imports := collectImports(rootPackage); importsLen != len(imports); imports = collectImports(rootPackage) {
		importsLen = len(imports)
		if err := removeUnusedImports(rootPackage, imports); err != nil {
			logrus.Errorf("Error removing unused dirs: %v", err)
		}
		if err := removeEmptyDirs(rootPackage); err != nil {
			logrus.Errorf("Error removing empty dirs: %v", err)
		}
	}
	return nil
}
