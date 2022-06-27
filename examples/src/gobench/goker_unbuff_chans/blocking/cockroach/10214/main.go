/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/10214
 * Buggy version: 7207111aa3a43df0552509365fdec741a53f873f
 * fix commit-id: 27e863d90ab0660494778f1c35966cc5ddc38e32
 * Flaky: 3/100
 * Description: This deadlock is caused by different order when acquiring
 * coalescedMu.Lock() and raftMu.Lock(). The fix is to refactor sendQueuedHeartbeats()
 * so that cockroachdb can unlock coalescedMu before locking raftMu.
 */
package main

import (
	"unsafe"
)

type Store struct {
	coalescedMu struct {
		mu                 chan bool
		heartbeatResponses []int
	}
	mu struct {
		replicas map[int]*Replica
	}
}

func (s *Store) sendQueuedHeartbeats() {
	s.coalescedMu.mu <- true // LockA acquire
	defer func() {
		<-s.coalescedMu.mu // LockA release
	}()
	for i := 0; i < len(s.coalescedMu.heartbeatResponses); i++ {
		s.sendQueuedHeartbeatsToNode() // LockB
	}
}

func (s *Store) sendQueuedHeartbeatsToNode() {
	for i := 0; i < len(s.mu.replicas); i++ {
		r := s.mu.replicas[i]
		r.reportUnreachable() // LockB
	}
}

type Replica struct {
	raftMu chan bool
	mu     chan bool
	store  *Store
}

func (r *Replica) reportUnreachable() {
	r.raftMu <- true // LockB acquire
	//+time.Sleep(time.Nanosecond)
	defer func() {
		<-r.raftMu
		// LockB release
	}()
}

func (r *Replica) tick() {
	r.raftMu <- true // LockB acquire
	defer func() {
		<-r.raftMu
	}()
	r.tickRaftMuLocked()
	// LockB release
}

func (r *Replica) tickRaftMuLocked() {
	r.mu <- true
	defer func() {
		<-r.mu
	}()
	if r.maybeQuiesceLocked() {
		return
	}
}
func (r *Replica) maybeQuiesceLocked() bool {
	for i := 0; i < 2; i++ {
		if !r.maybeCoalesceHeartbeat() {
			return true
		}
	}
	return false
}
func (r *Replica) maybeCoalesceHeartbeat() bool {
	msgtype := uintptr(unsafe.Pointer(r)) % 3
	switch msgtype {
	case 0, 1, 2:
		r.store.coalescedMu.mu <- true // LockA acquire
	default:
		return false
	}
	<-r.store.coalescedMu.mu // LockA release
	return true
}

func main() {
	store := &Store{
		coalescedMu: struct {
			mu                 chan bool
			heartbeatResponses []int
		}{
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
		},
	}
	responses := &store.coalescedMu.heartbeatResponses
	*responses = append(*responses, 1, 2)
	store.mu.replicas = make(map[int]*Replica)

	rp1 := &Replica{
		store: store,
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
	}
	rp2 := &Replica{
		store: store,
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
	}
	store.mu.replicas[0] = rp1
	store.mu.replicas[1] = rp2

	go func() {
		store.sendQueuedHeartbeats()
	}()

	go func() {
		rp1.tick()
	}()
}
