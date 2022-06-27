package main

type data struct {
	queue []struct{}
}

func (h *data) Pop() {
	h.queue = h.queue[0 : len(h.queue)-1]
}

type Interface interface {
	Pop()
}

func Pop(h Interface) {
	h.Pop()
}

type Heap struct {
	data *data
}

func (h *Heap) Pop() {
	Pop(h.data)
}
func (h *Heap) Len() int {
	return len(h.data.queue)
}

func NewWithRecorder() *Heap {
	return &Heap{
		data: &data{
			queue: []struct{}{
				struct{}{},
				struct{}{},
			},
		},
	}
}

type rwmutex struct {
	r chan bool
	w chan bool
}

type PriorityQueue struct {
	stop        chan struct{}
	lock        rwmutex
	podBackoffQ *Heap
	activeQ     *Heap
}

func (p *PriorityQueue) flushBackoffQCompleted() {
	p.lock.w <- true
	defer func() { <-p.lock.w }()
	p.podBackoffQ.Pop()

}

func NewPriorityQueue() *PriorityQueue {
	return &PriorityQueue{
		stop:        make(chan struct{}),
		activeQ:     NewWithRecorder(),
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
}

func createAndRunPriorityQueue() *PriorityQueue {
	q := NewPriorityQueue()
	q.Run()
	return q
}

func (p *PriorityQueue) Run() {
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

func JitterUntil(f func(), stopCh <-chan struct{}) {
	BackoffUntil(f, stopCh)
}

func Until(f func(), stopCh <-chan struct{}) {
	JitterUntil(f, stopCh)
}

type waitgroup struct {
	pool chan int
	wait chan bool
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
		p := createAndRunPriorityQueue()
		p.podBackoffQ.Len()
	}()
	<-wg.wait
}
