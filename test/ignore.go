// +build ignore

// package description
package trashtestfixture

import (
	"fmt"

	"github.com/Sirupsen/logrus"
)

func Foo() string {
	logrus.Fatal("This is a test")
	return fmt.Sprint("bar")
}
