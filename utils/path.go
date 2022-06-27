package utils

import (
	"flag"
	"fmt"
	"os"
	//"path/filepath"c
)

// MakePath returns a string based on whether a project name was provided or not
func MakePath() (path string) {
	path, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}
	args := flag.Args()
	if len(args) >= 1 {
		path = args[0]
		//path = filepath.Join("Goat", "examples", args[0])
	} else {
		path = "hello-world"
		//path = filepath.Join("Goat", "examples", "hello-world")
	}

	return
}
