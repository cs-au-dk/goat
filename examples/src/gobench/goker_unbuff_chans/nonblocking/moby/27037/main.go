package main

import (
	"fmt"
)

type waitgroup struct {
	wait chan bool
	pool chan int
}

func main() {
	wg := func() (wg waitgroup) {
		wg = waitgroup{
			pool: make(chan int),
			wait: make(chan bool),
		}

		go func() {
			count := 0

			for {
				select {
				// The WaitGroup may wait so long as the count is 0.
				case wg.wait <- true:
				// The first pooled goroutine will prompt the WaitGroup to wait
				// and disregard all sends on Wait until all pooled goroutines unblock.
				case x := <-wg.pool:
					count += x
					// TODO: Simulate counter dropping below 0 panics.
					for count > 0 {
						select {
						case x := <-wg.pool:
							count += x
						// Caller should receive on wg.Pool to decrement counter
						case wg.pool <- 0:
							count--
						}
					}
				}
			}
		}()

		return
	}()
	for i := 17; i <= 21; i++ {
		wg.pool <- 1
		go func() {
			defer func() { <-wg.pool }()
			_ = fmt.Sprintf("v1.%d", i)
		}()
	}
	<-wg.wait
}
