package main

import (
	"context"
)

type HostPriorityList []int

type DoWorkPieceFunc func(piece int)

type waitgroup struct {
	pool chan int
	wait chan bool
}

func ParallelizeUntil(ctx context.Context, workers, pieces int, doWorkPiece DoWorkPieceFunc) {
	var stop <-chan struct{}
	if ctx != nil {
		stop = ctx.Done()
	}

	toProcess := make(chan int, pieces)
	for i := 0; i < pieces; i++ {
		toProcess <- i
	}
	close(toProcess)

	if pieces < workers {
		workers = pieces
	}

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
	wg.pool <- workers
	for i := 0; i < workers; i++ {
		go func() {
			defer func() { <-wg.pool }()
			for piece := range toProcess {
				select {
				case <-stop:
					return
				default:
					doWorkPiece(piece)
				}
			}
		}()
	}
	<-wg.wait
}

func main() {
	priorityConfigs := append([]int{}, 1, 2, 3)
	results := make([]HostPriorityList, len(priorityConfigs), len(priorityConfigs))

	for i := range priorityConfigs {
		results[i] = make(HostPriorityList, 2)
	}
	processNode := func(index int) {
		for i := range priorityConfigs {
			if results[i][0] != 4 {
				results[i] = HostPriorityList{7, 8, 9}
			}
		}
	}
	ParallelizeUntil(context.Background(), 2, 2, processNode)
}
