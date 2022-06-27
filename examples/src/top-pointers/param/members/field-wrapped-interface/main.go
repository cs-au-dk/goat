package main

import "sync"

type A struct {
	mu sync.Mutex
}


func mkL() sync.Locker {
	var a A
	return &a.mu
}

func f(x sync.Locker) {
	if x == nil {
		x = mkL()
	}

	x.Lock()
	defer x.Unlock()
}

func main() {
	var a A
	f(&a.mu)
}
