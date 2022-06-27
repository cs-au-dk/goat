package main

type txBuffer struct {
	buckets map[string]struct{}
}

func (txb *txBuffer) reset() {
	for k, _ := range txb.buckets {
		delete(txb.buckets, k)
	}
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

type txReadBuffer struct{ txBuffer }

func (txr *txReadBuffer) Range() {
	_ = txr.buckets["1"]
}

type readTx struct {
	buf txReadBuffer
}

func (rt *readTx) reset() {
	rt.buf.reset()
}

func (rt *readTx) UnsafeRange() {
	rt.buf.Range()
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
	wg.pool <- 3
	go func() {
		defer func() { <-wg.pool }()
		txn := &readTx{
			buf: txReadBuffer{
				txBuffer{
					buckets: make(map[string]struct{}),
				},
			},
		}
		txn.buf.buckets["1"] = struct{}{}
		go func() {
			defer func() { <-wg.pool }()
			txn.reset()
		}()
		go func() {
			defer func() { <-wg.pool }()
			txn.UnsafeRange()
		}()
	}()
	<-wg.wait
}
