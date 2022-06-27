package main

import (
	"runtime"
	"time"
)

var x int = 20

func f() int {
	if true {
		done := make(chan struct{})
		if false {
			runtime.Goexit()
		}
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			runtime.Goexit()
		}
	}

	res := x * x
	return res
}

func main() {
	f()
}
