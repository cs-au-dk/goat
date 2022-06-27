package main

import "sync"

func main() {
	mu := sync.Mutex{}

	go func() {
		mu.Lock() //@ blocks
		_ = 1 + 1
		mu.Unlock()
	}()
	mu.Lock() //@ releases
}
