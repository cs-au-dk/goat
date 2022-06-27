package main

import "sync"

func main() {
	var mu sync.Mutex
	mu.Lock()

	go func() {
		go mu.Unlock()
	}()

	mu.Lock() //@ releases
}
