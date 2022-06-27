package main

import (
	"fmt"
	"time"
)

var (
	remoteURLLock = &remoteLock{m: make(map[string]chan bool)}
)

type remoteLock struct {
	mu rwmutex
	m  map[string]chan bool
}

func (l *remoteLock) URLLock(url string) {
	l.mu.w <- true
	if _, ok := l.m[url]; !ok {
		l.m[url] = func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}()
	}
	l.m[url] <- true
	<-l.mu.w
}

type rwmutex struct {
	w chan bool
	r chan bool
}

func (l *remoteLock) URLUnlock(url string) {
	l.mu.r <- true
	defer func() {
		<-l.mu.r
	}()
	if um, ok := l.m[url]; ok {
		<-um
	}
}

func resGetRemote(url string) error {
	remoteURLLock.URLLock(url)
	defer func() { remoteURLLock.URLUnlock(url) }()

	return nil
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func main() {
	url := "http://Foo.Bar/foo_Bar-Foo"
	for _ = range []bool{false, true} {
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
		for i := 0; i < 50; i++ {
			wg.pool <- 1
			go func(gor int) {
				defer func() {
					<-wg.pool
				}()
				for j := 0; j < 10; j++ {
					err := resGetRemote(url)
					if err != nil {
						fmt.Errorf("Error getting resource content: %s", err)
					}
					time.Sleep(300 * time.Nanosecond)
				}
			}(i)
		}
		<-wg.wait
	}
}
