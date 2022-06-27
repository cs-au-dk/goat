package main

import "sync"

var lock *sync.Mutex

func main() {
	lock.Lock() //@ blocks
	defer lock.Unlock()
}
