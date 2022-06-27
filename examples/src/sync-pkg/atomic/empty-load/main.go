package main

import "sync/atomic"

func main() {
	var v atomic.Value
	itf := v.Load()

	// This test is to make sure that we have enough precision to capture
	// that Load() on an unused atomic.Value definitely is nil.
	// It would be nice to have a better way to test this, but this
	// workaround works atm.

	ch := make(chan int, 1)

	if itf != nil {
		// Force fail the test
		ch <- 0
		// Introduce context sensitivity
		go func() {}()
	}

	ch <- 0 //@ releases
}
