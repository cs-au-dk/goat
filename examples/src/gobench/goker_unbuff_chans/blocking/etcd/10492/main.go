package main

import (
	"context"
	"time"
)

type Checkpointer func(ctx context.Context)

type rwmutex struct {
	w chan bool
	r chan bool
}

type lessor struct {
	mu                 rwmutex
	cp                 Checkpointer
	checkpointInterval time.Duration
}

func (le *lessor) Checkpoint() {
	le.mu.w <- true // block here
	defer func() {
		<-le.mu.w
	}()
}

func (le *lessor) SetCheckpointer(cp Checkpointer) {
	le.mu.w <- true
	defer func() {
		<-le.mu.w
	}()

	le.cp = cp
}

func (le *lessor) Renew() {
	le.mu.w <- true
	unlock := func() { <-le.mu.w }
	defer func() { unlock() }()

	if le.cp != nil {
		le.cp(context.Background())
	}
}
func main() {
	le := &lessor{
		checkpointInterval: 0,
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
	fakerCheckerpointer := func(ctx context.Context) {
		le.Checkpoint()
	}
	le.SetCheckpointer(fakerCheckerpointer)
	le.mu.w <- true
	<-le.mu.w
	le.Renew()
}
