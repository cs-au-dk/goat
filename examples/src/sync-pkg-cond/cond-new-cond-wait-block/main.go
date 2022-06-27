package main

import "sync"

func main() {
	c := sync.NewCond(&sync.Mutex{})

	go func() {
		c.L.Lock()
		c.Wait()
	}()

	c.L.Lock()
	c.Wait()
}
