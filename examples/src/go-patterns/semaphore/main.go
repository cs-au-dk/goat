package main

// GoLive: replaced errors (errors.New) with strings
//  also replaced "time.After" with default cases

import (
	//"errors"
	"time"
)

var (
	ErrNoTickets      = ("semaphore: could not aquire semaphore")
	ErrIllegalRelease = ("semaphore: can't release the semaphore without acquiring it first")
)

// Interface contains the behavior of a semaphore that can be acquired and/or released.
type Interface interface {
	Acquire() interface{}
	Release() interface{}
}

type implementation struct {
	sem     chan struct{}
	timeout time.Duration
}

func (s *implementation) Acquire() interface{} {
	select {
	case s.sem <- struct{}{}:
		return nil
	//case <-time.After(s.timeout):
	default:
		return ErrNoTickets
	}
}

func (s *implementation) Release() interface{} {
	select {
	case _ = <-s.sem:
		return nil
	//case <-time.After(s.timeout):
	default:
		return ErrIllegalRelease
	}
}

func New(tickets int, timeout time.Duration) Interface {
	return &implementation{
		sem:     make(chan struct{}, tickets),
		timeout: timeout,
	}
}

func main() {
	tickets, timeout := 1, 3*time.Second
	s := New(tickets, timeout)

	if err := s.Acquire(); err != nil {
		panic(err)
	}

	println("Do important work")

	if err := s.Release(); err != nil {
		panic(err)
	}
}
