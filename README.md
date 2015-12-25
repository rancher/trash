# Trash - Go ./vendor manager

Keeping the trash in your ./vendor dir to a minimum.

## How to use

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
