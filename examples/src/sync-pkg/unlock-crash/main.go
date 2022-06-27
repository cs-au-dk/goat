package main

import "sync"

func main() {
	mu := sync.Mutex{}
	mu.Unlock()
}
