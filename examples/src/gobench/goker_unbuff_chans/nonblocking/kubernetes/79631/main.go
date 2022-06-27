package main

type heapData struct {
	items map[string]struct{}
}

func (h *heapData) Pop() {
	delete(h.items, "1")
}

type Interface interface {
	Pop()
}

func Pop(h Interface) {
	h.Pop()
}

type Heap struct {
	data *heapData
}

func (h *Heap) Pop() {
	Pop(h.data)
}

func (h *Heap) Get() {
	h.GetByKey()
}

func (h *Heap) GetByKey() {
	_ = h.data.items["1"]
}

func NewWithRecorder() *Heap {
	return &Heap{
		data: &heapData{
			items: make(map[string]struct{}),
		},
	}
}

type rwmutex struct {
	w chan bool
	r chan bool
}

type PriorityQueue struct {
	stop        chan struct{}
	lock        rwmutex
	podBackoffQ *Heap
}

func (p *PriorityQueue) flushBackoffQCompleted() {
	p.lock.w <- true
	defer func() { <-p.lock.w }()
	p.podBackoffQ.Pop()

}

func NewPriorityQueue() *PriorityQueue {
	return NewPriorityQueueWithClock()
}

func NewPriorityQueueWithClock() *PriorityQueue {
	pg := &PriorityQueue{
		stop:        make(chan struct{}),
		podBackoffQ: NewWithRecorder(),
		lock: func() (lock rwmutex) {
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
	pg.run()
	return pg
}

func (p *PriorityQueue) run() {
	go Until(p.flushBackoffQCompleted, p.stop)
}

func BackoffUntil(f func(), stopCh <-chan struct{}) {
	for {
		select {
		case <-stopCh:
			return
		default:
		}

		func() {
			f()
		}()

		select {
		case <-stopCh:
			return
		}
	}
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func JitterUntil(f func(), stopCh <-chan struct{}) {
	BackoffUntil(f, stopCh)
}

func Until(f func(), stopCh <-chan struct{}) {
	JitterUntil(f, stopCh)
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
		<-wg.pool
		q := NewPriorityQueue()
		q.podBackoffQ.Get()
	}()
	<-wg.wait
}
