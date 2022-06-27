package main

import "sync"

func main() {
	var mu sync.RWMutex

	go func(l sync.Locker) {
		l.Lock() //@ releases
	}(mu.RLocker())

	mu.RLock() //@ releases
}
