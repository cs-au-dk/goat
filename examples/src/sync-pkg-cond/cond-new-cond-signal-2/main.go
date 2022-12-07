package main

import "sync"

func main() {
	c := sync.NewCond(&sync.Mutex{})

	c.L.Lock()

	go func() {
		c.Signal()
	}()

	go func() {
		// No unlock after wait, so this may block
		c.L.Lock() //@ blocks
	}()

	// This may block if Signal occurs before Wait
	c.Wait() //@ blocks
}
