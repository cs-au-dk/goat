package main

import "sync"

func main() {
	var mu sync.Mutex
	mu.Lock()

	go mu.Unlock()

	mu.Lock() //@ releases
}
