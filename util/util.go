package util

import (
	"bufio"
	"os/exec"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
)

func CmdOutLines(cmd *exec.Cmd) <-chan string {
	r := make(chan string, 1000)
	out, err := cmd.StdoutPipe()
	if err != nil {
		logrus.Fatalf("Could not obtain stdout of `%s`", strings.Join(cmd.Args, " "))
	}
	scanner := bufio.NewScanner(out)
	if err := cmd.Start(); err != nil {
		logrus.Fatalf("Could not start `%s`", strings.Join(cmd.Args, " "))
	}
	go func() {
		defer close(r)
		defer cmd.Wait()
		for scanner.Scan() {
			r <- scanner.Text()
		}
	}()
	return r
}

func Merge(cs ...<-chan string) <-chan string {
	out := make(chan string)
	wg := sync.WaitGroup{}
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

func OneOff(s string) <-chan string {
	c := make(chan string, 1)
	defer close(c)
	c <- s
	return c
}
