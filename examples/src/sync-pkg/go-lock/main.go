package main

import "sync"

func main() {
	l := &sync.Mutex{}

	go l.Lock()
	go l.Lock()

	l.Unlock()
	l.Unlock()
}
