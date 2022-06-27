package main

import "sync"

func main() {
	mu := sync.Mutex{}
	mu.Lock() //@ releases
	mu.Lock() //@ blocks
}
