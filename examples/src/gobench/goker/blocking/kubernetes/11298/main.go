package main

import (
	"sync"
	"time"
)

type Signal <-chan struct{}

func After(f func()) Signal {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		if f != nil {
			f()
		}
	}()
	return Signal(ch)
}

func Until(f func(), period time.Duration, stopCh <-chan struct{}) {
	if f == nil {
		return
	}
	for {
		select {
		case <-stopCh:
			return
		default:
		}
		func() {
			f()
		}()
		select {
		case <-stopCh:
		case <-time.After(period):
		}
	}

}

type notifier struct {
	lock sync.Mutex
	cond *sync.Cond
}

// abort will be closed no matter what
func (n *notifier) serviceLoop(abort <-chan struct{}) {
	n.lock.Lock()
	defer n.lock.Unlock()
	for {
		select {
		case <-abort:
			return
		default:
			ch := After(func() {
				n.cond.Wait() //@ blocks, fn
			})
			select { //@ releases,fp
			case <-abort:
				n.cond.Signal()
				<-ch //@ blocks, fn
				return
			case <-ch:
			}
		}
	}
}

// abort will be closed no matter what
func Notify(abort <-chan struct{}) {
	n := &notifier{}
	n.cond = sync.NewCond(&n.lock)
	finished := After(func() {
		Until(func() {
			for {
				select {
				case <-abort:
					return
				default:
					func() {
						n.lock.Lock() //@ releases, fp
						defer n.lock.Unlock()
						n.cond.Signal()
					}()
				}
			}
		}, 0, abort)
	})
	Until(func() { n.serviceLoop(finished) }, 0, abort)
}
func main() {
	done := make(chan struct{})
	notifyDone := After(func() { Notify(done) })
	go func() {
		defer close(done)
		time.Sleep(300 * time.Nanosecond)
	}()
	// This is not described in the bug, but is also a true positive
	<-notifyDone //@ blocks
}

// The result we get for this program is really bad.
// I think it's because goroutines are all spawned from the same site
// and we have a goroutine bound of 1, so the goroutine spawned in the
// After call at line 56 is not spawned.
// TODO: Should we perform over-approximation for skipped goroutine spawns?
