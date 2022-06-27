package main

import (
	"time"
)

type Stopper struct {
	Done chan bool
}

func (s *Stopper) ShouldStop() <-chan bool {
	return s.Done
}

type EventMembershipChangeCommitted struct {
	Callback func()
}
type MultiRaft struct {
	stopper      *Stopper
	Events       chan interface{}
	callbackChan chan func()
}

// sendEvent can be invoked many times
func (m *MultiRaft) sendEvent(event interface{}) {
	/// FIX:
	/// Let event append a event queue instead of pending here
	select { //@ blocks(goro2)
	case m.Events <- event: // Waiting for events consumption
	case <-m.stopper.ShouldStop():
	}
}

type state struct {
	*MultiRaft
}

func (s *state) start() {
	for {
		select {
		case <-s.stopper.ShouldStop():
			return
		case cb := <-s.callbackChan:
			cb()
		default:
			s.handleWriteResponse()
		}
	}
}
func (s *state) handleWriteResponse() {
	s.processCommittedEntry()
}

func (s *state) processCommittedEntry() {
	s.sendEvent(&EventMembershipChangeCommitted{
		Callback: func() {
			select { //@ analysis(false), blocks(goro1)
			case s.callbackChan <- func() { // Waiting for callbackChan consumption
				time.Sleep(time.Nanosecond)
			}:
			case <-s.stopper.ShouldStop():
			}
		},
	})
}

type Store struct {
	multiraft *MultiRaft
}

func (s *Store) processRaft() {
	for {
		select {
		case e := <-s.multiraft.Events:
			var callback func()
			switch e := e.(type) {
			case *EventMembershipChangeCommitted:
				callback = e.Callback
				if callback != nil {
					callback() // Waiting for callbackChan consumption
				}
			}
		case <-s.multiraft.stopper.ShouldStop():
			return
		}
	}
}

func NewStoreAndState() (*Store, *state) {
	stopper := &Stopper{
		Done: make(chan bool), //@ chan(stopperDone)
	}
	mltrft := &MultiRaft{
		stopper:      stopper,
		Events:       make(chan interface{}), //@ chan(multiraftEvents)
		callbackChan: make(chan func()),      //@ chan(multiraftCbChan)
	}
	st := &state{mltrft}
	s := &Store{mltrft}
	return s, st
}

//@ goro(goro1, true, _root, g1)
//@ goro(goro2, true, _root, g2)
func main() {
	s, st := NewStoreAndState()
	go s.processRaft() // G1 //@ go(g1)
	go st.start()      // G2 //@ go(g2)
}
