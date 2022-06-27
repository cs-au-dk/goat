package main

import (
	"sync"
)

type Plugin struct {
	activated    bool
	activateWait *sync.Cond
}

type plugins struct {
	sync.Mutex
	plugins map[int]*Plugin
}

func (p *Plugin) waitActive() {
	p.activateWait.L.Lock()
	for !p.activated {
		p.activateWait.Wait() //@ blocks
	}
	p.activateWait.L.Unlock()
}

type extpointHandlers struct {
	sync.RWMutex
	extpointHandlers map[int]struct{}
}

var (
	storage  = plugins{plugins: make(map[int]*Plugin)}
	handlers = extpointHandlers{extpointHandlers: make(map[int]struct{})}
)

func Handle() {
	handlers.Lock()
	for _, p := range storage.plugins {
		p.activated = false
	}
	handlers.Unlock()
}

func testActive(p *Plugin) {
	done := make(chan struct{})
	go func() {
		p.waitActive()
		close(done)
	}()
	// If p.waitActive blocks then this is also a true positive
	<-done //@ blocks
}

func main() {
	p := &Plugin{activateWait: sync.NewCond(&sync.Mutex{})}
	storage.plugins[0] = p

	testActive(p)
	Handle()
	testActive(p)
}
