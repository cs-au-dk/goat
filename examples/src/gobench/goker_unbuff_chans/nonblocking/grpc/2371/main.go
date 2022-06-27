package main

import (
	"time"
)

type ccBalancerWrapper struct {
	cc               *ClientConn
	resolverUpdateCh chan struct{}
}

func (ccb *ccBalancerWrapper) handleResolvedAddrs() {
	select {
	case <-ccb.resolverUpdateCh:
	default:
	}
	ccb.resolverUpdateCh <- struct{}{}
}

func newCCBalancerWrapper(cc *ClientConn) *ccBalancerWrapper {
	ccb := &ccBalancerWrapper{
		cc:               cc,
		resolverUpdateCh: make(chan struct{}, 1),
	}
	return ccb
}

type ccResolverWrapper struct {
	cc *ClientConn
}

func (ccr *ccResolverWrapper) start() {
	go ccr.watcher()
}

func (ccr *ccResolverWrapper) watcher() {
	ccr.cc.handleServiceConfig()
}

func newCCResolverWrapper(cc *ClientConn) *ccResolverWrapper {
	ccr := &ccResolverWrapper{
		cc: cc,
	}
	return ccr
}

type rwmutex struct {
	w chan bool
	r chan bool
}

type ClientConn struct {
	mu              rwmutex
	balancerWrapper *ccBalancerWrapper
	resolverWrapper *ccResolverWrapper
}

func (cc *ClientConn) handleServiceConfig() {
	cc.mu.w <- true
	cc.balancerWrapper.handleResolvedAddrs()
	<-cc.mu.w
}

func (cc *ClientConn) Close() {
	cc.mu.w <- true
	cc.resolverWrapper = nil
	cc.balancerWrapper = nil
	<-cc.mu.w
}

func Dial() *ClientConn {
	return DialContext()
}

func DialContext() *ClientConn {
	cc := &ClientConn{
		mu: func() (lock rwmutex) {
			lock = rwmutex{
				w: make(chan bool),
				r: make(chan bool),
			}

			go func() {
				rCount := 0

				// As long as all locks are free, both a reader
				// and a writer may acquire the lock
				for {
					select {
					// If a writer acquires the lock, hold it until released
					case <-lock.w:
						lock.w <- false
						// If a reader acquires the lock, step into read-mode.
					case <-lock.r:
						// Increment the reader count
						rCount++
						// As long as not all readers are released, stay in read-mode.
						for rCount > 0 {
							select {
							// One reader released the lock
							case lock.r <- false:
								rCount--
								// One reader acquired the lock
							case <-lock.r:
								rCount++
							}
						}
					}
				}
			}()

			return lock
		}(),
	}

	cc.resolverWrapper = newCCResolverWrapper(cc)
	cc.balancerWrapper = newCCBalancerWrapper(cc)

	cc.resolverWrapper.start()

	return cc
}

func main() {

	for i := 0; i < 10; i++ {
		cc := Dial()

		go cc.Close()
	}

	time.Sleep(100 * time.Millisecond)
}
