package main

import (
	"context"

	"time"
)

type Proxy interface {
	IsLive() bool
}

type TestProxy struct {
	live func() bool
}

func (tp TestProxy) IsLive() bool {
	if tp.live == nil {
		return true
	}
	return tp.live()
}

type Agent interface {
	Run(ctx context.Context)
	Restart()
}

type exitStatus int

type agent struct {
	proxy        Proxy
	mu           chan bool
	statusCh     chan exitStatus
	currentEpoch int
	activeEpochs map[int]struct{}
}

func (a *agent) Run(ctx context.Context) {
	for {
		select {
		case status := <-a.statusCh:
			a.mu <- true
			delete(a.activeEpochs, int(status))
			active := len(a.activeEpochs)
			<-a.mu
			if active == 0 {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (a *agent) Restart() {
	a.mu <- true
	defer func() {
		<-a.mu
	}()

	a.waitUntilLive()
	a.currentEpoch++
	a.activeEpochs[a.currentEpoch] = struct{}{}

	go a.runWait(a.currentEpoch)
}

func (a *agent) runWait(epoch int) {
	a.statusCh <- exitStatus(epoch)
}

func (a *agent) waitUntilLive() {
	if len(a.activeEpochs) == 0 {
		return
	}

	interval := time.NewTicker(30 * time.Nanosecond)
	timer := time.NewTimer(100 * time.Nanosecond)
	defer func() {
		interval.Stop()
		timer.Stop()
	}()

	if a.proxy.IsLive() {
		return
	}

	for {
		select {
		case <-timer.C:
			return
		case <-interval.C:
			if a.proxy.IsLive() {
				return
			}
		}
	}
}

func NewAgent(proxy Proxy) Agent {
	return &agent{
		proxy: proxy,
		mu: func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}(),
		statusCh:     make(chan exitStatus),
		activeEpochs: make(map[int]struct{}),
	}
}
func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	neverLive := func() bool {
		return false
	}

	a := NewAgent(TestProxy{live: neverLive})
	go func() { a.Run(ctx) }()

	a.Restart()
	go a.Restart()

	time.Sleep(200 * time.Nanosecond)
}
