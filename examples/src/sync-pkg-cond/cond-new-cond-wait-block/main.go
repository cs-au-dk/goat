package main

import "sync"

func main() {
	c := sync.NewCond(&sync.Mutex{})

	go func() {
		c.L.Lock() //@ releases
		c.Wait() //@ blocks
	}()

	c.L.Lock() //@ releases
	c.Wait() //@ blocks
}
