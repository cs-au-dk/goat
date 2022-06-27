package main

import (
	"runtime"
)

type token struct{}

type request struct {
	lock     chan bool
	accepted chan bool
}

type Breaker struct {
	pendingRequests chan token
	activeRequests  chan token
}

func (b *Breaker) Maybe(thunk func()) bool {
	var t token
	select {
	default:
		// Pending request queue is full.  Report failure.
		return false
	case b.pendingRequests <- t:
		// Pending request has capacity.
		// Wait for capacity in the active queue.
		b.activeRequests <- t
		// Defer releasing capacity in the active and pending request queue.
		defer func() { <-b.activeRequests; <-b.pendingRequests }()
		// Do the thing.
		thunk()
		// Report success
		return true
	}
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func (b *Breaker) concurrentRequest() request {
	runtime.Gosched()

	r := request{lock: func() (lock chan bool) {
		lock = make(chan bool)
		go func() {
			for {
				<-lock
				lock <- false
			}
		}()
		return
	}(), accepted: make(chan bool, 1)}
	r.lock <- true
	var start = func() (wg waitgroup) {
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
	start.pool <- 1
	go func() { // G2, G3
		<-start.pool
		ok := b.Maybe(func() {
			r.lock <- true // Will block on locked mutex.
			<-r.lock
		})
		r.accepted <- ok
	}()
	<-start.wait // Ensure that the go func has had a chance to execute.
	return r
}

// Perform n requests against the breaker, returning mutexes for each
// request which succeeded, and a slice of bools for all requests.
func (b *Breaker) concurrentRequests(n int) []request {
	requests := make([]request, n)
	for i := range requests {
		requests[i] = b.concurrentRequest()
	}
	return requests
}

func NewBreaker(queueDepth, maxConcurrency int32) *Breaker {
	return &Breaker{
		pendingRequests: make(chan token, queueDepth+maxConcurrency),
		activeRequests:  make(chan token, maxConcurrency),
	}
}

func unlock(req request) {
	<-req.lock
	// Verify that function has completed
	ok := <-req.accepted
	// Requeue for next usage
	req.accepted <- ok
}

func unlockAll(requests []request) {
	for _, lc := range requests {
		unlock(lc)
	}
}

//
// G1                           G2                      G3
// b.concurrentRequests(2)
// b.concurrentRequest()
// r.lock.Lock()
//                                                      start.Done()
// start.Wait()
// b.concurrentRequest()
// r.lock.Lock()
//                              start.Done()
// start.Wait()
// unlockAll(locks)
// unlock(lc)
// req.lock.Unlock()
// ok := <-req.accepted
//                              b.Maybe()
//                              b.activeRequests <- t
//                              thunk()
//                              r.lock.Lock()
//                                                      b.Maybe()
//                                                      b.activeRequests <- t
// ----------------------------G1,G2,G3 deadlock-----------------------------
//
func main() {
	b := NewBreaker(1, 1)

	locks := b.concurrentRequests(2) // G1
	unlockAll(locks)
}
