package main

import (
	"pkg-with-test-bloat/sub"
)

type T int
func (T) PublicMethod() {
	sub.Fun(func() {
		println("Hello World")
	})
}

func main() {
	var t T = 5
	t.PublicMethod()
}
