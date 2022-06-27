package main

import "sync"

func main() {
	c := sync.Cond{
		L: &sync.Mutex{},
	}

	go func() {
		c.Signal()
	}()

	c.L.Lock() //@ releases
	c.Wait() //@ blocks
}
