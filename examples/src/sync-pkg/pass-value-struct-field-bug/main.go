package main

import "sync"

type M struct {
	mu sync.Mutex
}

func main() {
	mu := M{}
	go func(pmu M) {
		pmu.mu.Unlock()
	}(mu)
	mu.mu.Lock()
	mu.mu.Lock() //@ blocks
}
