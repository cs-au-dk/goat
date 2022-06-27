package main

import (
	"time"
)

type Source interface {
	Start()
	Stop()
}

type fsSource struct {
	donec chan struct{}
}

func (s *fsSource) Start() {
	go func() {
		for {
			select {
			case <-s.donec:
				return
			}
		}
	}()
}

func (s *fsSource) Stop() {
	close(s.donec)
	s.donec = nil
}

func newFsSource() *fsSource {
	return &fsSource{
		donec: make(chan struct{}),
	}
}

func New() Source {
	return newFsSource()
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func main() {
	var wg = func() (wg waitgroup) {
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
	wg.pool <- 1
	go func() {
		defer func() { <-wg.pool }()
		s := New()
		s.Start()
		s.Stop()
		time.Sleep(5 * time.Millisecond)
	}()
	<-wg.wait
}
