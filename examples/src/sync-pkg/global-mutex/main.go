package main

import "sync"

var lock sync.Mutex

func main() {
	lock.Lock() //@ releases
	defer lock.Unlock()
}
