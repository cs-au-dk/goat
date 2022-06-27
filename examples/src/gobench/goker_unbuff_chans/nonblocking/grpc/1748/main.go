package main

import (
	"time"
)

var minConnectTimeout = 10 * time.Second

var balanceMutex = func() (lock chan bool) {
	lock = make(chan bool)
	go func() {
		for {
			<-lock
			lock <- false
		}
	}()
	return
}() // We add this for avoiding other data race

type Balancer interface {
	HandleResolvedAddrs()
}

type Builder interface {
	Build(cc balancer_ClientConn) Balancer
}

func newPickfirstBuilder() Builder {
	return &pickfirstBuilder{}
}

type pickfirstBuilder struct{}

func (*pickfirstBuilder) Build(cc balancer_ClientConn) Balancer {
	return &pickfirstBalancer{cc: cc}
}

type SubConn interface {
	Connect()
}

type balancer_ClientConn interface {
	NewSubConn() SubConn
}

type pickfirstBalancer struct {
	cc balancer_ClientConn
	sc SubConn
}

func (b *pickfirstBalancer) HandleResolvedAddrs() {
	b.sc = b.cc.NewSubConn()
	b.sc.Connect()
}

type pickerWrapper struct {
	mu chan bool
}

type acBalancerWrapper struct {
	mu chan bool
	ac *addrConn
}

type addrConn struct {
	cc   *ClientConn
	acbw SubConn
	mu   chan bool
}

func (ac *addrConn) resetTransport() {
	_ = minConnectTimeout
}

func (ac *addrConn) transportMonitor() {
	ac.resetTransport()
}

func (ac *addrConn) connect() {
	go func() {
		ac.transportMonitor()
	}()
}

func (acbw *acBalancerWrapper) Connect() {
	acbw.mu <- true
	defer func() { <-acbw.mu }()
	acbw.ac.connect()
}

func newPickerWrapper() *pickerWrapper {
	return &pickerWrapper{
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

type ClientConn struct {
	mu chan bool
}

func (cc *ClientConn) switchBalancer() {
	builder := newPickfirstBuilder()
	newCCBalancerWrapper(cc, builder)
}

func (cc *ClientConn) newAddrConn() *addrConn {
	return &addrConn{cc: cc, mu: func() (lock chan bool) {
		lock = make(chan bool)
		go func() {
			for {
				<-lock
				lock <- false
			}
		}()
		return
	}()}
}

type ccBalancerWrapper struct {
	cc       *ClientConn
	balancer Balancer
}

func (ccb *ccBalancerWrapper) watcher() {
	for i := 0; i < 10; i++ {
		balanceMutex <- true
		if ccb.balancer != nil {
			<-balanceMutex
			ccb.balancer.HandleResolvedAddrs()
		} else {
			<-balanceMutex
		}
	}
}

func (ccb *ccBalancerWrapper) NewSubConn() SubConn {
	ac := ccb.cc.newAddrConn()
	acbw := &acBalancerWrapper{
		ac: ac,
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
	acbw.ac.mu <- true
	ac.acbw = acbw
	<-acbw.ac.mu
	return acbw
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func newCCBalancerWrapper(cc *ClientConn, b Builder) {
	ccb := &ccBalancerWrapper{cc: cc}
	go ccb.watcher()
	balanceMutex <- true
	defer func() { <-balanceMutex }()
	ccb.balancer = b.Build(ccb)
}

func main() {
	var wg = func() (wg waitgroup) {
		wg = waitgroup{
			pool: make(chan int),
			wait: make(chan bool),
		}

		go func() {
			count := 0

			for {
				select {
				// The WaitGroup may wait so long as the count is 0.
				case wg.wait <- true:
				// The first pooled goroutine will prompt the WaitGroup to wait
				// and disregard all sends on Wait until all pooled goroutines unblock.
				case x := <-wg.pool:
					count += x
					// TODO: Simulate counter dropping below 0 panics.
					for count > 0 {
						select {
						case x := <-wg.pool:
							count += x
						// Caller should receive on wg.Pool to decrement counter
						case wg.pool <- 0:
							count--
						}
					}
				}
			}
		}()

		return
	}()
	wg.pool <- 1
	go func() {
		defer func() { <-wg.pool }()
		mctBkp := minConnectTimeout
		// Call this only after transportMonitor goroutine has ended.
		defer func() {
			minConnectTimeout = mctBkp
		}()
		cc := &ClientConn{}
		cc.switchBalancer()
	}()
	<-wg.wait
}
