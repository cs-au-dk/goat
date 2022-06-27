package main

import "sync"

func main() {
	var l sync.Locker = &sync.Mutex{}
	l.Lock() //@ releases
	defer l.Unlock()
}
