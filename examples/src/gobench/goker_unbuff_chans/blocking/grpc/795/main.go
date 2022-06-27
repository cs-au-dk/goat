package main

type Server struct {
	mu    chan bool
	drain bool
}

func (s *Server) GracefulStop() {
	s.mu <- true
	if s.drain == true {
		<-s.mu
		return
	}
	s.drain = true
} // Missing Unlock

func (s *Server) Serve() {
	s.mu <- true
	<-s.mu
}

func NewServer() *Server {
	return &Server{
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
