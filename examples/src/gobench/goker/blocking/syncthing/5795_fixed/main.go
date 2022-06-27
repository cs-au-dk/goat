package main

import (
	"sync"
)

type message interface{}

type ClusterConfig struct{}

type Model interface {
	ClusterConfig(message)
}

type TestModel struct {
	ccFn func()
}

func (t *TestModel) ClusterConfig(msg message) {
	if t.ccFn != nil {
		t.ccFn()
	}
}

func newTestModel() *TestModel {
	return &TestModel{}
}

type Connection interface {
	Start()
	Close()
}

type rawConnection struct {
	receiver Model

	inbox                 chan message
	dispatcherLoopStopped chan struct{}
	closed                chan struct{}
	closeOnce             sync.Once
}

func (c *rawConnection) Start() {
	go c.readerLoop()
	go func() { //@ go(go1)
		c.dispatcherLoop()
	}()
}

func (c *rawConnection) readerLoop() {
	for {
		select { // Orphans are detected at this control location.
		case <-c.closed:
			return
		default:
		}
	}
}

func (c *rawConnection) dispatcherLoop() {
	defer close(c.dispatcherLoopStopped)
	var msg message
	for {
		// Orphans are detected at this control location.
		// The reason is the same as in the unfixed version.
		// We need to be able to prove that the Once function is run.
		select { //@ releases, fp
		case msg = <-c.inbox:
		case <-c.closed:
			return
		}
		switch msg := msg.(type) {
		case *ClusterConfig:
			c.receiver.ClusterConfig(msg)
		default:
			return
		}
	}
}

func (c *rawConnection) internalClose() {
	c.closeOnce.Do(func() {
		close(c.closed)
		<-c.dispatcherLoopStopped //@ releases(closer)
	})
}

func (c *rawConnection) Close() {
	go c.internalClose() //@ go(goclose) // FIX: go c.internalClose()
	// FIX implies unbounded goroutine spawning:
	// main -calls-> Start -spawns-> dispatcherLoop {
	// - if note closed and message is ClusterConfig, -calls-> ClusterConfig of
	// rawConnection in a loop. This -calls-> ccFn of TestModel, which calls c.Close()
}

func NewConnection(receiver Model) Connection {
	return &rawConnection{
		dispatcherLoopStopped: make(chan struct{}),
		closed:                make(chan struct{}),
		inbox:                 make(chan message),
		receiver:              receiver,
	}
}

//@ goro(closer, true, _root, go1, goclose)

func main() {
	m := newTestModel()
	c := NewConnection(m).(*rawConnection)
	m.ccFn = func() {
		c.Close()
	}

	c.Start()
	c.inbox <- &ClusterConfig{}

	// Due to the Once bug this blocks in one (impossible) setting
	<-c.dispatcherLoopStopped //@ releases, fp
}
