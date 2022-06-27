package main

// This example tests how select works. Note that ch1 is never selected.

// GoLive: replaced fmt.Printf with println

import (
	//"fmt"
)

func main() {
	ch0 := make(chan int)
	ch1 := make(chan int)

	go func() {
		ch0 <- 42 // @ analysis(true) // Requires refinement of C
	}()

	// Blocking
	select { //@ analysis(true)
	case x := <-ch0:
		println("Result is", x)
	case ch1 <- 2: // This is a mismatch, no receive on ch1
	}
}
