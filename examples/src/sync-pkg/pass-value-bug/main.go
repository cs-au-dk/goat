package main

import "sync"

func main() {
	mu := sync.Mutex{}
	go func(pmu sync.Mutex) {
		pmu.Unlock()
	}(mu)
	mu.Lock()
	mu.Lock() //@ blocks
}
