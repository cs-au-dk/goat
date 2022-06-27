package main

import (
	"time"
)

type Conn interface {
	Write(b []byte)
}

type pipe struct {
	wrMu chan bool
}

func (p *pipe) Write(b []byte) {
	p.wrMu <- true
	defer func() { <-p.wrMu }()
	b = b[1:]
}

func Pipe() Conn {
	return &pipe{
		wrMu: func() (lock chan bool) {
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

func main() {
	srv := Pipe()
	tests := [][2][]byte{
		{
			[]byte("GET /foo\nHost: /var/run/docker.sock\nUser-Agent: Docker\r\n\r\n"),
			[]byte("GET /foo\nHost: \r\nConnection: close\r\nUser-Agent: Docker\r\n\r\n"),
		},
		{
			[]byte("GET /foo\nHost: /var/run/docker.sock\nUser-Agent: Docker\nFoo: Bar\r\n"),
			[]byte("GET /foo\nHost: \r\nConnection: close\r\nUser-Agent: Docker\nFoo: Bar\r\n"),
		},
	}
	for _, pair := range tests {
		go func() {
			srv.Write(pair[0])
		}()
	}
	time.Sleep(10 * time.Millisecond)
}
