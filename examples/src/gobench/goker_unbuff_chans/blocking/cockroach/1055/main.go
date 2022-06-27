package main

import (
	"sync/atomic"
)

type Stopper struct {
	stopper chan struct{}
	stop    struct {
		Pool chan int
		Wait chan bool
	}
	mu       chan bool
	draining int32
	drain    struct {
		Pool chan int
		Wait chan bool
	}
}

func (s *Stopper) AddWorker() {
	s.stop.Pool <- 1
}

func (s *Stopper) ShouldStop() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.stopper
}

func (s *Stopper) SetStopped() {
	if s != nil {
		<-s.stop.Pool
	}
}

func (s *Stopper) Quiesce() {
	s.mu <- true
	defer func() {
		<-s.mu
	}()
	s.draining = 1
	<-s.drain.Wait
	s.draining = 0
}

func (s *Stopper) Stop() {
	s.mu <- true
	defer func() {
		<-s.mu
	}()
	atomic.StoreInt32(&s.draining, 1)
	<-s.drain.Wait
	close(s.stopper)
	<-s.stop.Wait
}

func (s *Stopper) StartTask() bool {
	if atomic.LoadInt32(&s.draining) == 0 {
		s.mu <- true
		defer func() {
			<-s.mu
		}()
		s.drain.Pool <- 1
		return true
	}
	return false
}

func NewStopper() *Stopper {
	return &Stopper{
		stopper: make(chan struct{}),
		stop: func() (wg struct {
			Pool chan int
			Wait chan bool
		}) {
			wg = struct {
				Pool chan int
				Wait chan bool
			}{
				Pool: make(chan int),
				Wait: make(chan bool),
			}

			go func() {
				count := 0

				for {
					select {
					// The WaitGroup may wait so long as the count is 0.
					case <-wg.Wait:
					// The first pooled goroutine will prompt the WaitGroup to wait
					// and disregard all sends on Wait until all pooled goroutines unblock.
					case x := <-wg.Pool:
						count += x
						// TODO: Simulate counter dropping below 0 panics.
						for count > 0 {
							select {
							case x := <-wg.Pool:
								count += x
							// Caller should receive on wg.Pool to decrement counter
							case wg.Pool <- 0:
								count--
							}
						}
					}
				}
			}()

			return
		}(),
		mu: func() chan bool {
			ch := make(chan bool)
			go func() {
				for {
					<-ch
					ch <- false
				}
			}()
			return ch
		}(),
		drain: func() (wg struct {
			Pool chan int
			Wait chan bool
		}) {
			wg = struct {
				Pool chan int
				Wait chan bool
			}{
				Pool: make(chan int),
				Wait: make(chan bool),
			}

			go func() {
				count := 0

				for {
					select {
					// The WaitGroup may wait so long as the count is 0.
					case <-wg.Wait:
					// The first pooled goroutine will prompt the WaitGroup to wait
					// and disregard all sends on Wait until all pooled goroutines unblock.
					case x := <-wg.Pool:
						count += x
						// TODO: Simulate counter dropping below 0 panics.
						for count > 0 {
							select {
							case x := <-wg.Pool:
								count += x
							// Caller should receive on wg.Pool to decrement counter
							case wg.Pool <- 0:
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
	var stoppers []*Stopper
	for i := 0; i < 3; i++ {
		stoppers = append(stoppers, NewStopper())
	}

	for i := range stoppers {
		s := stoppers[i]
		s.AddWorker()
		go func() {
			s.StartTask()
			<-s.ShouldStop()
			s.SetStopped()
		}()
	}

	done := make(chan struct{})
	go func() {
		for _, s := range stoppers {
			s.Quiesce()
		}
		for _, s := range stoppers {
			s.Stop()
		}
		close(done)
	}()

	<-done
}
