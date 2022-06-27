package main

import "sync"

func main() {
	var mu sync.Mutex
	mu.Lock()

	go func() {
		mu.Unlock()
	}()

	mu.Lock() //@ releases
}
