package main

import "sync"

type Mu struct {
	sync.Mutex
}

type Mu2 struct {
	mu sync.Mutex
}

func main() {
	(&sync.Mutex{}).Lock()
	var muu sync.Mutex
	muu = sync.Mutex{}
	muu.Lock()
	var nilmu sync.Mutex
	nilmu.Lock()
	var mu Mu
	if func() bool { return true }() {
		mu = Mu{}
	} else {
		mu = Mu{}
	}
	mu.Lock()
	var mu3 *Mu
	mu3 = new(Mu)
	mu3.Lock()

	mu2 := Mu2{}
	mu2.mu.Lock()
	((&Mu2{}).mu.Lock())
	mu.Unlock()
	mu.Lock()
}
