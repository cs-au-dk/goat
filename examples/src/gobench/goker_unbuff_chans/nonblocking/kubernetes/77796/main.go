package main

import (
	"time"
)

type cacheWatcher int

type rwmutex struct {
	w chan bool
	r chan bool
}

type Cacher struct {
	rwmutex
	watcherBuffer []*cacheWatcher
}

func (c *Cacher) startDispatching() {
	c.w <- true
	defer func() { <-c.w }()

	c.watcherBuffer = c.watcherBuffer[:0]
}

func (c *Cacher) dispatchEvent() {
	c.startDispatching()
	for _ = range c.watcherBuffer {
	}
}

func (c *Cacher) dispatchEvents() {
	c.dispatchEvent()
}

func NewCacherFromConfig() *Cacher {
	cacher := &Cacher{}
	go cacher.dispatchEvents()
	return cacher
}

func newTestCacher() *Cacher {
	return NewCacherFromConfig()
}

func main() {
	cacher := newTestCacher()
	for i := 0; i < 3; i++ {
		go func() {
			cacher.dispatchEvent()
		}()
		time.Sleep(10 * time.Millisecond)
	}
}
