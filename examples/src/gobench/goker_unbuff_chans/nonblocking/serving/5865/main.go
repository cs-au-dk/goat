package main

import (
	"sync"
)

type revisionWatcher struct {
	destsCh chan struct{}
}

func (rw *revisionWatcher) run() {
	defer close(rw.destsCh)
}

type revisionBackendsManager struct {
	revisionWatchersMux sync.RWMutex
}

func newRevisionWatcher(destsCh chan struct{}) *revisionWatcher {
	return &revisionWatcher{destsCh: destsCh}
}

func (rbm *revisionBackendsManager) endpointsUpdated() {
	rw := rbm.getOrCreateRevisionWatcher()
	rw.destsCh <- struct{}{}
}

func (rbm *revisionBackendsManager) getOrCreateRevisionWatcher() *revisionWatcher {
	rbm.revisionWatchersMux <- true
	defer rbm.revisionWatchersMux.Unlock()

	destsCh := make(chan struct{})
	rw := newRevisionWatcher(destsCh)
	go rw.run()

	return rw
}

func newRevisionBackendsManagerWithProbeFrequency() *revisionBackendsManager {
	rbm := &revisionBackendsManager{}
	return rbm
}

func main() {
	rbm := newRevisionBackendsManagerWithProbeFrequency()

	// Simplified code in the RealTestSuite
	func() {
		rbm.endpointsUpdated()
	}()
}
