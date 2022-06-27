package main

import "sync"

func main() {
	c := sync.NewCond(&sync.Mutex{})

	go func() {
		c.Wait()
	}()

	c.Wait()
}
