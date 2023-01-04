package utils

import (
	"flag"
	"fmt"
	"os"
)

// MakePath returns a string based on whether a Go package path was provided or not.
// The first non-flag argument passed to Goat is the target package.
// If no path is provided, it defaults to "hello-world".
func MakePath() (path string) {
	path, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}
	args := flag.Args()
	if len(args) >= 1 {
		path = args[0]
	} else {
		path = "hello-world"
	}

	return
}
