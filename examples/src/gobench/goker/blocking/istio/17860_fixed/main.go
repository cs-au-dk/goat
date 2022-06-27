package main

import (
	"context"
	"errors"

	"sync"
	"time"
)

type Proxy interface {
	Run(int) error
	IsLive() bool
}

type TestProxy struct {
	run  func(int) error
	live func() bool
}

func (tp TestProxy) IsLive() bool {
	if tp.live == nil {
		return true
	}
	return tp.live()
}

func (tp TestProxy) Run(epoch int) error {
	if tp.run == nil {
		return nil
	}
	return tp.run(epoch)
}

type Agent interface {
	Run(ctx context.Context)
	Restart()
}

type exitStatus int

type agent struct {
	proxy        Proxy
	restartMutex sync.Mutex
	mutex        sync.Mutex
	statusCh     chan exitStatus
	currentEpoch int
	activeEpochs map[int]struct{}
}

func (a *agent) Run(ctx context.Context) {
	for {
		select {
		case status := <-a.statusCh:
			a.mutex.Lock()
			delete(a.activeEpochs, int(status))
			//active := len(a.activeEpochs)
			a.mutex.Unlock()
			/*
			if active == 0 {
				return
			}
			*/
		case <-ctx.Done():
			return
		}
	}
}

func (a *agent) Restart() {
	// Only allow one restart to execute at a time.
	a.restartMutex.Lock()
	defer a.restartMutex.Unlock()

	// Protect access to internal state
	a.mutex.Lock()

	hasActiveEpoch := len(a.activeEpochs) > 0
	activeEpoch := a.currentEpoch

	// Increment the latest running epoch
	a.currentEpoch++
	a.activeEpochs[a.currentEpoch] = struct{}{}

	// Unlock before the wait to avoid delaying envoy exit logic
	a.mutex.Unlock()

	// Wait for the previous epoch to go live (if one exists) before performing hot restart.
	if hasActiveEpoch {
		a.waitUntilLive(activeEpoch)
	}

	go a.runWait(a.currentEpoch) //@ go(wait)
}

func (a *agent) runWait(epoch int) {
	_ = a.proxy.Run(epoch)
	// I do not understand how the fix works, but I think it relies a lot on mutexes.
	// In any case our analysis still reports that both g1 and g2 can block here.
	a.statusCh <- exitStatus(epoch) // @ releases(g1), releases(g2), fp
}

func (a *agent) isActive(epoch int) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()
	_, ok := a.activeEpochs[epoch]
	return ok
}

func (a *agent) waitUntilLive(epoch int) {
	if len(a.activeEpochs) == 0 {
		return
	}

	interval := time.NewTicker(30 * time.Nanosecond)
	timer := time.NewTimer(100 * time.Nanosecond)

	isDone := func() bool {
		if !a.isActive(epoch) {
			return true
		}

		return a.proxy.IsLive()
	}

	defer func() {
		interval.Stop()
		timer.Stop()
	}()

	if isDone() {
		return
	}

	for {
		select {
		case <-timer.C:
			return
		case <-interval.C:
			if isDone() {
				return
			}
		}
	}
}

func NewAgent(proxy Proxy) Agent {
	return &agent{
		proxy:        proxy,
		statusCh:     make(chan exitStatus),
		activeEpochs: make(map[int]struct{}),
		currentEpoch: -1,
	}
}

//@ goro(main, true, _root)
//@ goro(g1, true, _root, restart, wait)
//@ goro(g2, true, _root, wait)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	epoch0Exit := make(chan error)
	epoch1Started := make(chan struct{}, 1)
	start := func(epoch int) error {
		switch epoch {
		case 0:
			// The first epoch just waits for the exit error.
			return <-epoch0Exit
		case 1:
			// Indicate that the second epoch was started.
			close(epoch1Started)
		}
		<-ctx.Done()
		return nil
	}
	neverLive := func() bool {
		return false
	}

	a := NewAgent(TestProxy{run: start, live: neverLive})
	go func() { a.Run(ctx) }()

	a.Restart()
	go a.Restart() //@ go(restart)

	// Trigger the first epoch to exit
	epoch0Exit <- errors.New("fake") //@ releases(main)

	select {
	case <-epoch1Started: //@ releases(main)
		// Started
		break
	}

	// Moved cancel call into separate goroutine to prevent spurious context cycle
	go cancel()
}
