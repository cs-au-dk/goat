package main

import "sync"

func outside(*sync.Cond)

func main() {
	var mu sync.Mutex
	cond := sync.NewCond(&mu)

	go func() {
		mu.Lock() //@ releases, fp
		defer mu.Unlock()
	}()

	mu.Lock() //@ releases
	outside(cond)
	cond.Wait() //@ blocks
}
