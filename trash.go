package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
	"github.com/imikushin/trash/conf"
)

func exit(err error) {
	if err != nil {
		logrus.Fatal(err)
	}
}

func main() {
	app := cli.NewApp()
	app.Author = "@imikushin"
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
			Usage: "Keep all downloaded vendor code",
		},
		cli.BoolFlag{
			Name:  "debug, d",
			Usage: "Debug logging",
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

	if err := os.Chdir(dir); err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}

	if err := vendor(dir, trashFile); err != nil {
		return err
	}
	if !keep {
		return cleanup(dir)
	}

	return nil
}

func vendor(dir, trashFile string) error {
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

	trashDir := path.Join(os.Getenv("HOME"), ".trash-cache")
	os.MkdirAll(trashDir, 0755)
	os.Setenv("GOPATH", trashDir)

	for _, i := range trashConf.Imports {
		if err := prepareCache(trashDir, i); err != nil {
			logrus.WithFields(logrus.Fields{"err": err, "i": i}).Error("Failed to prepare trash")
			return err
		}
	}

	for _, i := range trashConf.Imports {
		if err := checkout(trashDir, i); err != nil {
			logrus.WithFields(logrus.Fields{"err": err, "i": i}).Error("Failed to checkout")
			return err
		}
	}

	vendorDir := path.Join(dir, "vendor")
	os.RemoveAll(vendorDir)
	os.MkdirAll(vendorDir, 0755)

	for _, i := range trashConf.Imports {
		if err := cpy(vendorDir, trashDir, i); err != nil {
			logrus.WithFields(logrus.Fields{"err": err, "i": i}).Error("Failed to copy dep")
			return err
		}
	}

	return nil
}

func prepareCache(trashDir string, i conf.Import) error {
	logrus.WithFields(logrus.Fields{"trashDir": trashDir, "i": i}).Debug("entering prepareCache")
	os.Chdir(trashDir)
	repoDir := path.Join(trashDir, "src", i.Package)
	if err := checkGitRepo(trashDir, repoDir, i); err != nil {
		logrus.WithFields(logrus.Fields{"err": err}).Error("checkGitRepo failed")
		return err
	}
	return nil
}

func checkout(trashDir string, i conf.Import) error {
	logrus.WithFields(logrus.Fields{"trashDir": trashDir, "i": i}).Debug("entering checkout")
	repoDir := path.Join(trashDir, "src", i.Package)
	if err := os.Chdir(repoDir); err != nil {
		logrus.Errorf("Could not change to dir '%s'", repoDir)
		return err
	}
	logrus.Debugf("Checkout: `git checkout -f --detach %s`", i.Version)
	if bytes, err := exec.Command("git", "checkout", "-f", "--detach", i.Version).CombinedOutput(); err != nil {
		logrus.Warnf("`git checkout -f --detach %s` failed:\n%s", i.Version, bytes)
		if err := fetch(i); err != nil {
			logrus.WithFields(logrus.Fields{"i": i}).Errorf("fetch failed")
			return err
		}
		logrus.Debugf("Retrying!: `git checkout -f --detach %s`", i.Version)
		if bytes, err := exec.Command("git", "checkout", "-f", "--detach", i.Version).CombinedOutput(); err != nil {
			logrus.Errorf("`git checkout -f --detach %s` failed:\n%s", i.Version, bytes)
			return err
		}
	}
	return nil
}

func cpy(vendorDir, trashDir string, i conf.Import) error {
	repoDir := path.Join(trashDir, "src", i.Package)
	target, _ := path.Split(path.Join(vendorDir, i.Package))
	os.MkdirAll(target, 0755)
	if bytes, err := exec.Command("cp", "-a", repoDir, target).CombinedOutput(); err != nil {
		logrus.Errorf("`cp -a %s %s` failed:\n%s", repoDir, target, bytes)
		return err
	}
	return nil
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
		goGet := exec.Command("go", "get", "-d", "-f", "-u", i.Package)
		if bytes, err := goGet.CombinedOutput(); err != nil {
			logrus.WithFields(logrus.Fields{"err": err}).Warnf("`go get -d -f -u %s` returned err:\n%s", i.Package, bytes)
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
	logrus.Debug("running `git fetch -f -t %s` for %s", repo, i.Package)
	if bytes, err := exec.Command("git", "fetch", "-f", "-t", repo).CombinedOutput(); err != nil {
		logrus.Errorf("`git fetch -f -t %s` failed:\n%s", repo, bytes)
		return err
	}
	return nil
}

func cleanup(dir string) error {
	return nil
}
