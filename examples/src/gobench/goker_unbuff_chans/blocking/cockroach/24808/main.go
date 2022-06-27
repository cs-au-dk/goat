package main

import (
	"context"
)

type Compactor struct {
	ch chan struct{}
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

type Stopper struct {
	stop    waitgroup
	stopper chan struct{}
}

func (s *Stopper) RunWorker(ctx context.Context, f func(context.Context)) {
	s.stop.pool <- 1
	go func() {
		defer func() {
			<-s.stop.pool
		}()
		f(ctx)
	}()
}

func (s *Stopper) ShouldStop() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.stopper
}

func (s *Stopper) Stop() {
	close(s.stopper)
}

func NewStopper() *Stopper {
	s := &Stopper{
		stopper: make(chan struct{}),
		stop: func() (wg waitgroup) {
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
		}(),
	}
	return s
}

func NewCompactor() *Compactor {
	return &Compactor{ch: make(chan struct{}, 1)}
}

func (c *Compactor) Start(ctx context.Context, stopper *Stopper) {
	c.ch <- struct{}{} // Blocks
	stopper.RunWorker(ctx, func(ctx context.Context) {
		for {
			select {
			case <-stopper.ShouldStop():
				return
			case <-c.ch:
			}
		}
	})
}

func main() {
	stopper := NewStopper()
	defer stopper.Stop()

	compactor := NewCompactor()
	compactor.ch <- struct{}{} // Fills buffer

	compactor.Start(context.Background(), stopper)
}
