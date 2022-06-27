package main

import "sync"

func spawn(a *sync.Mutex) {
	a.Lock() //@ releases
}

func main() {
	go spawn(&sync.Mutex{})
	go spawn(&sync.Mutex{})
}
