package main

import "sync"

// Scenario 1:
// G1 							G2
// lock							|
//	|								V
//	V							blocks

// Scenario 2:
// G1 							G2
//  | 						 lock
//	|								|
// blocks						V

func main() {
	mu := sync.RWMutex{}

	go func() {
		mu.Lock() //@ blocks
	}()

	mu.Lock() //@ blocks
}
