package main

import (
	"sync/atomic"
)

type ConsumerStatus uint32

const (
	NeedMoreRows ConsumerStatus = iota
	DrainRequested
	ConsumerClosed
)

const rowChannelBufSize = 16
const outboxBufRows = 16

type rowSourceBase struct {
	consumerStatus ConsumerStatus
}

func (rb *rowSourceBase) consumerClosed() {
	atomic.StoreUint32((*uint32)(&rb.consumerStatus), uint32(ConsumerClosed))
}

type RowChannelMsg int

type RowChannel struct {
	rowSourceBase
	dataChan chan RowChannelMsg
}

func (rc *RowChannel) ConsumerClosed() {
	rc.consumerClosed()
	select {
	case <-rc.dataChan:
	default:
	}
}

func (rc *RowChannel) Push() ConsumerStatus {
	consumerStatus := ConsumerStatus(
		atomic.LoadUint32((*uint32)(&rc.consumerStatus)))
	switch consumerStatus {
	case NeedMoreRows:
		rc.dataChan <- RowChannelMsg(0)
	case DrainRequested:
	case ConsumerClosed:
	}
	return consumerStatus
}

func (rc *RowChannel) InitWithNumSenders() {
	rc.initWithBufSizeAndNumSenders(rowChannelBufSize)
}

func (rc *RowChannel) initWithBufSizeAndNumSenders(chanBufSize int) {
	rc.dataChan = make(chan RowChannelMsg, chanBufSize)
}

type outbox struct {
	RowChannel
}

func (m *outbox) init() {
	m.RowChannel.InitWithNumSenders()
}

func (m *outbox) start(wg *waitgroup) {
	if wg != nil {
		wg.pool <- 1
	}
	go m.run(wg)
}

func (m *outbox) run(wg *waitgroup) {
	m.mainLoop()
	if wg != nil {
		<-wg.pool
	}
}

func (m *outbox) mainLoop() {
	return
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func main() {
	outbox := &outbox{}
	outbox.init()

	wg := func() (wg waitgroup) {
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
	for i := 0; i < outboxBufRows; i++ {
		outbox.Push()
	}

	blockedPusherWg := func() (wg waitgroup) {
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
	blockedPusherWg.pool <- 1
	go func() {
		outbox.Push()
		<-blockedPusherWg.pool
	}()

	outbox.start(&wg)

	<-wg.wait
	outbox.RowChannel.Push()
}
