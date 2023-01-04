package main

import "sync"

func f(cnd *sync.Cond) {
	cnd.L.Lock() //@ releases

	go func() {
		cnd.L.Lock() //@ releases
		defer cnd.L.Unlock()
		cnd.Signal() //@ releases
	}()

	cnd.Wait() //@ releases, fp
}

func main() {
	var mu sync.Mutex
	cnd := &sync.Cond{L: &mu}
	f(cnd)
}
