package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
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

	logrus.Debug("Hello trash!")

	return nil
}
