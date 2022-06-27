package main

import (
	"pkg-with-test/another-pkg"
	"testing"
)

func TestHi(t *testing.T) {
	Hi()
}

func TestHejJorden(t *testing.T) {
	anotherPkg.LocalFunc()
}

func TestFatal(t *testing.T) {
	ch := make(chan int)
	go func() {
		<-ch //@ blocks
	}()

	t.Fatal("Oh no")
	ch <- 10
}
