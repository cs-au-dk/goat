package main

import "sync"

func main() {
	mu := sync.Mutex{}
	go func() {
		mu.Unlock()
		mu.Unlock()
	}()
	mu.Lock()
	mu.Lock() //@ releases
}
