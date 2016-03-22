package util

import (
	"bufio"
	"os/exec"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
)

type Packages map[string]bool

func (p Packages) Merge(x Packages) Packages {
	for k := range x {
		p[k] = true
	}
	return p
}

func ChanPackages(f func() Packages) <-chan Packages {
	c := make(chan Packages, 1)
	go func() {
		defer close(c)
		c <- f()
	}()
	return c
}

func CmdOutLines(cmd *exec.Cmd) <-chan string {
	r := make(chan string, 1000)
	out, err := cmd.StdoutPipe()
	if err != nil {
		logrus.Fatalf("Could not obtain stdout of `%s`: %s", strings.Join(cmd.Args, " "), err)
	}
	scanner := bufio.NewScanner(out)
	go func() {
		defer close(r)
		defer out.Close()
		defer cmd.Wait()
		for scanner.Scan() {
			r <- scanner.Text()
		}
	}()
	if err := cmd.Start(); err != nil {
		logrus.Fatalf("Could not start `%s`: %s", strings.Join(cmd.Args, " "), err)
	}
	return r
}

func MergePackagesChans(cs ...<-chan Packages) <-chan Packages {
	out := make(chan Packages)
	wg := sync.WaitGroup{}
	wg.Add(len(cs))
	for _, c := range cs {
		go func(c <-chan Packages) {
			defer wg.Done()
			for s := range c {
				out <- s
			}
		}(c)
	}
	go func() {
		defer close(out)
		wg.Wait()
	}()
	return out
}

func MergeStrChans(cs ...<-chan string) <-chan string {
	out := make(chan string)
	wg := &sync.WaitGroup{}
	wg.Add(len(cs))
	for _, c := range cs {
		go func(c <-chan string) {
			defer wg.Done()
			for s := range c {
				out <- s
			}
		}(c)
	}
	go func() {
		defer close(out)
		wg.Wait()
	}()
	return out
}

func OneStr(s string) <-chan string {
	c := make(chan string, 1)
	defer close(c)
	c <- s
	return c
}
