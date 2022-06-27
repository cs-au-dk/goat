package main

import (
	"context"
	"time"
)

type addrConn struct {
	mu    chan bool
	cc    *ClientConn
	addr  Address
	dopts dialOptions
	down  func()
}

func (ac *addrConn) tearDown() {
	ac.mu <- true
	defer func() {
		<-ac.mu
	}()
	if ac.down != nil {
		ac.down()
		ac.down = nil
	}
}

func (ac *addrConn) resetTransport() {
	ac.mu <- true
	if ac.cc.dopts.balancer != nil {
		ac.down = ac.cc.dopts.balancer.Up(ac.addr)
	}
	<-ac.mu
}

type rwmutex struct {
	w chan bool
	r chan bool
}

type ClientConn struct {
	dopts dialOptions
	mu    rwmutex
	conns map[Address]*addrConn
}

func (cc *ClientConn) lbWatcher() {
	for addrs := range cc.dopts.balancer.Notify() {
		var (
			add []Address
			del []*addrConn
		)
		cc.mu.w <- true
		for _, a := range addrs {
			if _, ok := cc.conns[a]; !ok {
				add = append(add, a)
			}
		}

		for k, c := range cc.conns {
			var keep bool
			for _, a := range addrs {
				if k == a {
					keep = true
					break
				}
			}
			if !keep {
				del = append(del, c)
				delete(cc.conns, c.addr)
			}
		}
		<-cc.mu.w
		for _, a := range add {
			cc.resetAddrConn(a)
		}
		for _, c := range del {
			c.tearDown()
		}
	}
}

func (cc *ClientConn) resetAddrConn(addr Address) {
	ac := &addrConn{
		cc:    cc,
		addr:  addr,
		dopts: cc.dopts,
	}
	cc.mu.w <- true
	if cc.conns == nil {
		<-cc.mu.w
		return
	}
	cc.conns[ac.addr] = ac
	<-cc.mu.w
	go func() {
		ac.resetTransport()
	}()
}

func (cc *ClientConn) Close() {
	cc.mu.w <- true
	conns := cc.conns
	cc.conns = nil
	<-cc.mu.w
	if cc.dopts.balancer != nil {
		cc.dopts.balancer.Close()
	}
	for _, ac := range conns {
		ac.tearDown()
	}
}

type dialOptions struct {
	balancer Balancer
}
type DialOption func(*dialOptions)

func Dial(opts ...DialOption) *ClientConn {
	return DialContext(context.Background(), opts...)
}

func DialContext(ctx context.Context, opts ...DialOption) *ClientConn {
	cc := &ClientConn{
		conns: make(map[Address]*addrConn),
		mu: func() (lock rwmutex) {
			lock = rwmutex{
				w: make(chan bool),
				r: make(chan bool),
			}

			go func() {
				rCount := 0

				// As long as all locks are free, both a reader
				// and a writer may acquire the lock
				for {
					select {
					// If a writer acquires the lock, hold it until released
					case <-lock.w:
						lock.w <- false
						// If a reader acquires the lock, step into read-mode.
					case <-lock.r:
						// Increment the reader count
						rCount++
						// As long as not all readers are released, stay in read-mode.
						for rCount > 0 {
							select {
							// One reader released the lock
							case lock.r <- false:
								rCount--
								// One reader acquired the lock
							case <-lock.r:
								rCount++
							}
						}
					}
				}
			}()

			return lock
		}(),
	}
	for _, opt := range opts {
		opt(&cc.dopts)
	}
	go cc.lbWatcher()
	return cc
}

type Balancer interface {
	Up(addr Address) (down func())
	Notify() <-chan []Address
	Close()
}

type Address int

type simpleBalancer struct {
	addrs    []Address
	notifyCh chan []Address
	mu       rwmutex
	closed   bool
	pinAddr  Address
}

func (b *simpleBalancer) Up(addr Address) func() {
	b.mu.w <- true
	defer func() {
		<-b.mu.w
	}()

	if b.closed {
		return func() {}
	}

	if b.pinAddr == 0 {
		b.pinAddr = addr
		b.notifyCh <- []Address{addr}
	}

	return func() {
		b.mu.w <- true
		if b.pinAddr == addr {
			b.pinAddr = 0
			b.notifyCh <- b.addrs
		}
		<-b.mu.w
	}
}

func (b *simpleBalancer) Notify() <-chan []Address {
	return b.notifyCh
}

func (b *simpleBalancer) Close() {
	b.mu.w <- true
	defer func() {
		<-b.mu.w
	}()
	if b.closed {
		return
	}
	b.closed = true
	close(b.notifyCh)
	b.pinAddr = 0
}

func newSimpleBalancer() *simpleBalancer {
	notifyCh := make(chan []Address, 1)
	addrs := make([]Address, 3)
	for i := 0; i < 3; i++ {
		addrs[i] = Address(i)
	}
	notifyCh <- addrs
	return &simpleBalancer{
		addrs:    addrs,
		notifyCh: notifyCh,
		mu: func() (lock rwmutex) {
			lock = rwmutex{
				w: make(chan bool),
				r: make(chan bool),
			}

			go func() {
				rCount := 0

				// As long as all locks are free, both a reader
				// and a writer may acquire the lock
				for {
					select {
					// If a writer acquires the lock, hold it until released
					case <-lock.w:
						lock.w <- false
						// If a reader acquires the lock, step into read-mode.
					case <-lock.r:
						// Increment the reader count
						rCount++
						// As long as not all readers are released, stay in read-mode.
						for rCount > 0 {
							select {
							// One reader released the lock
							case lock.r <- false:
								rCount--
								// One reader acquired the lock
							case <-lock.r:
								rCount++
							}
						}
					}
				}
			}()

			return lock
		}(),
	}
}

func WithBalancer(b Balancer) DialOption {
	return func(o *dialOptions) {
		o.balancer = b
	}
}
func main() {
	sb := newSimpleBalancer()
	conn := Dial(WithBalancer(sb))

	// Avoid data race addrConn.tearDown() and simpleBalancer.Close()
	time.Sleep(300 * time.Nanosecond)

	closec := make(chan struct{})
	go func() {
		defer close(closec)
		sb.Close()
	}()
	go conn.Close()
	<-closec
}
