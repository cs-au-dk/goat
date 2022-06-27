package main

import (
	"time"
)

type Stopper struct {
	stopper chan struct{}
	stop    waitgroup
	mu      chan bool
}

func (s *Stopper) RunWorker(f func()) {
	s.stop.pool <- 1
	go func() {
		defer func() { <-s.stop.pool }()
		f()
	}()
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func (s *Stopper) SetStopped() {
	if s != nil {
		<-s.stop.pool
	}
}

func (s *Stopper) Stop() {
	close(s.stopper)
	<-s.stop.wait
	s.mu <- true
	defer func() { <-s.mu }()
}

type server struct {
	mu      chan bool
	stopper *Stopper
}

func (s *server) Gossip() {
	s.mu <- true
	defer func() { <-s.mu }()
	s.stopper.RunWorker(func() {
		s.gossipSender()
	})
}

func (s *server) gossipSender() {
	s.mu <- true
	defer func() { <-s.mu }()
}

func NewStopper() *Stopper {
	return &Stopper{
		stopper: make(chan struct{}),
		mu: func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}(),
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
}

func main() {
	stopper := NewStopper()
	defer stopper.Stop()
	s := &server{
		stopper: stopper,
		mu: func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}(),
	}
	for i := 0; i < 2; i++ {
		go s.Gossip()
	}
	time.Sleep(time.Millisecond)
}
