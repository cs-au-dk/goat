package main

import (
	"sync"
)

type Server struct {
	mu    sync.Mutex
	drain bool
}

func (s *Server) GracefulStop() {
	s.mu.Lock()
	if s.drain == true {
		s.mu.Lock() //@ blocks
		return
	}
	s.drain = true
} // Missing Unlock

func (s *Server) Serve() {
	s.mu.Lock() //@ blocks
	s.mu.Unlock()
}

func NewServer() *Server {
	return &Server{}
}

type test struct {
	srv *Server
}

func (te *test) startServer() {
	s := NewServer()
	te.srv = s
	go s.Serve()
}

func newTest() *test {
	return &test{}
}

func testServerGracefulStopIdempotent() {
	te := newTest()

	te.startServer()

	for i := 0; i < 3; i++ {
		te.srv.GracefulStop()
	}
}

func main() {
	testServerGracefulStopIdempotent()
}
