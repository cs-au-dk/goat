package main

import (
	"sync"
)

type Gossip struct {
	mu     sync.Mutex
	closed bool
}

func (g *Gossip) bootstrap() {
	for {
		g.mu.Lock() //@ releases
		if g.closed {
			g.mu.Unlock()
			break
		}
		g.mu.Unlock()
	}
}

func (g *Gossip) manage() {
	for {
		g.mu.Lock() //@ releases
		if g.closed {
			g.mu.Unlock()
			break
		}
		g.mu.Unlock()
	}
}
func main() {
	g := &Gossip{
		closed: true,
	}
	go func() {
		g.bootstrap()
		g.manage()
	}()
}
