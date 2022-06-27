package main

import "os"

func main() {
	ch := make(chan int, 3)

	go func() {
		for {
			ch <- 10
		}
	}()

	x := []bool{true, false}
	var b bool
	for _, y := range x {
		b = y
	}
	if b {
		os.Exit(1)
	}
}
