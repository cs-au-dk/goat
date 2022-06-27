package main

import (
	"io/fs"
	"math/rand"
)

func f(err error) string {
	var errStr string
	if rand.Int() == 0 {
		errStr = err.Error()
	} else {
		// errStr = "boo"
	}

	return errStr + errStr
}

func main() {
	f(new(fs.PathError))

}
