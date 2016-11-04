# Trash - Go ./vendor manager

Keeping the trash in your ./vendor dir to a minimum.

## How to use

Make sure you're using go1.6 or later version.

 1. Download and extract [latest release](https://github.com/rancher/trash/releases/latest) to your PATH.
    Alternatively, install or update current development version with `go get -u github.com/rancher/trash`.
 2. Copy `vendor.conf` file to your project and edit to your needs.
 3. Run `trash`.

`vendor.conf` (in your project root dir) specifies the revisions (git tags or commits, or branches - if you're drunk) of the libraries to be fetched, checked out and copied to ./vendor dir. For example:
```
github.com/rancher/trash

github.com/Sirupsen/logrus                      v0.8.7    https://github.com/imikushin/logrus.git
github.com/codegangsta/cli                      b5232bb
github.com/cloudfoundry-incubator/candiedyaml   5a459c2
```

Or, in YML format:
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

I really liked [glide](https://github.com/Masterminds/glide), it's like a *real* package manager: specify what you need, run `glide up` and enjoy your updated libraries. But it didn't help with a couple problems I had:

- All necessary library code should be vendored and checked into project repo (as imposed by the project policy)
- Unnecessary code should be removed ~~for great justice~~ for smaller git checkouts and faster `docker build`

I'd been slightly reluctant to the idea of writing it, but apparently the world needed another package manager: "Come on, it's just going to be 300 (okay, it's ~600) lines of Go!" Thanks to [@ibuildthecloud](https://github.com/ibuildthecloud) for the idea.

## Help

For the world's convenience, `trash` can detect glide.yaml (and glide.yml, as well as trash.yaml) and use that instead of vendor.conf (and you can Force it to use any other file). Just in case, here's the program help:

```
$ trash -h
NAME:
   trash - Vendor imported packages and throw away the trash!

USAGE:
   trash [global options] command [command options] [arguments...]

VERSION:
   v0.2.5

AUTHOR(S):
   @imikushin, @ibuildthecloud

COMMANDS:
     help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --file value, -f value       Vendored packages list (default: "vendor.conf")
   --directory value, -C value  The directory in which to run, --file is relative to this (default: ".")
   --keep, -k                   Keep all downloaded vendor code (preserving .git dirs)
   --update, -u                 Update vendored packages, add missing ones
   --debug, -d                  Debug logging
   --cache value                Cache directory (default: "/Users/ivan/.trash-cache") [$TRASH_CACHE]
   --help, -h                   show help
   --version, -v                print the version
```
