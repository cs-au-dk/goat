package main

import "sync"

func main() {
	c := sync.NewCond(&sync.Mutex{})

	go func() {
		c.Signal()
	}()

	c.L.Lock()
	c.Wait()
}
