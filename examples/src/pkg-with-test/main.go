package main

import (
	anotherPkg "pkg-with-test/another-pkg"
)

func Hi() {
	ch := make(chan int)
	go func() {
		ch <- 10
	}()
	<-ch
}

func Hi2() {
	anotherPkg.LocalFunc()
	ch := make(chan int)
	go func() {
		ch <- 10
	}()
	<-ch
}

func main() {
	Hi()
}
