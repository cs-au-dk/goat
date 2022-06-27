package main

import "sync"

type M struct {
	sync.Mutex
}

func main() {
	mu := M{}
	go func(pmu M) {
		pmu.Unlock()
	}(mu)
	mu.Lock()
	mu.Lock() //@ blocks
}
