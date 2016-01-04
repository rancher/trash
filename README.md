# Trash - Go ./vendor manager

[![Join the chat at https://gitter.im/imikushin/trash](https://badges.gitter.im/imikushin/trash.svg)](https://gitter.im/imikushin/trash?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

Keeping the trash in your ./vendor dir to a minimum.

## How to use

Make sure you're using Go 1.5+ and **GO15VENDOREXPERIMENT=1** env var is exported.

 0. `go get github.com/imikushin/trash`
 1. Copy `trash.yml` file to your project and edit to your needs.
 2. Run `trash`

`trash.yml` (in your project root dir) specifies the revisions (git tags or commits, or branches - if you're drunk) of the libraries to be fetched, checked out and copied to ./vendor dir. For example:
```yaml
import:
- package: github.com/Sirupsen/logrus               # package name
  version: v0.8.7                                   # tag or commit
  repo:    https://github.com/imikushin/logrus.git  # (optional) git URL

- package: github.com/codegangsta/cli
  version: b5232bb2934f606f9f27a1305f1eea224e8e8b88

- package: github.com/cloudfoundry-incubator/candiedyaml
  version: 55a459c2d9da2b078f0725e5fb324823b2c71702
```

Run `trash` to populate ./vendor directory and remove unnecessary files. Run `trash --keep` to keep *all* checked out files in ./vendor dir.

## Inspiration

I really like [glide](https://github.com/Masterminds/glide), it's like a *real* package manager: you specify what you need, run `glide up` and enjoy your updated libraries. But it doesn't help solving the 2 problems I've been having lately:

- All necessary library code should be vendored and checked into project repo (as imposed by the project policy)
- Unnecessary code should be removed ~~for great justice~~ for smaller git checkouts and faster `docker build`

I've been reluctant to the idea of writing `trash`, but apparently the world needs another package manager: come on, it's just going to be 300 (okay, it's ~600) lines of Go! Thanks to [@ibuildthecloud](https://github.com/ibuildthecloud) for the idea.

## Help

For the world's convenience, `trash` can detect glide.yaml (and glide.yml, as well as trash.yaml) and use that instead of trash.yml (and you can Force it to use any other file). Just in case, here's the program help:

```
$ trash -h
NAME:
   trash - Vendor imported packages and throw away the trash!

USAGE:
   trash [global options] command [command options] [arguments...]

VERSION:
   0.0.0

AUTHOR(S):
   @imikushin

COMMANDS:
   help, h	Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --file, -f "trash.yml"   Vendored packages list
   --directory, -C "."      The directory in which to run, --file is relative to this
   --keep, -k               Keep all downloaded vendor code
   --debug, -d              Debug logging
   --help, -h               show help
   --version, -v            print the version
```