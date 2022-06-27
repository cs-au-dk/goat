package main

import (
	"time"
)

const unschedulableQTimeInterval = 60 * time.Second

type Pod string

type PodInfo struct {
	Pod       Pod
	Timestamp time.Time
}

type UnschedulablePodsMap struct {
	podInfoMap map[string]*PodInfo
	keyFunc    func(Pod) string
}

func (u *UnschedulablePodsMap) addOrUpdate(pInfo *PodInfo) {
	podID := u.keyFunc(pInfo.Pod)
	u.podInfoMap[podID] = pInfo
}

func GetPodFullName(pod Pod) string {
	return string(pod)
}

func newUnschedulablePodsMap() *UnschedulablePodsMap {
	return &UnschedulablePodsMap{
		podInfoMap: make(map[string]*PodInfo),
		keyFunc:    GetPodFullName,
	}
}

type rwmutex struct {
	w chan bool
	r chan bool
}

type PriorityQueue struct {
	stop           <-chan struct{}
	lock           rwmutex
	unschedulableQ *UnschedulablePodsMap
}

func (p *PriorityQueue) flushUnschedulableQLeftover() {
	p.lock.w <- true
	defer func() { <-p.lock.w }()

	for _, pInfo := range p.unschedulableQ.podInfoMap {
		_ = pInfo.Timestamp
	}
}

func (p *PriorityQueue) run() {
	go Until(p.flushUnschedulableQLeftover, p.stop)
}

func (p *PriorityQueue) newPodInfo(pod Pod) *PodInfo {
	return &PodInfo{
		Pod:       pod,
		Timestamp: time.Now(),
	}
}

func NewPriorityQueueWithClock(stop <-chan struct{}) *PriorityQueue {
	pq := &PriorityQueue{
		stop: stop,
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
		unschedulableQ: newUnschedulablePodsMap(),
	}
	pq.run()
	return pq
}

func NewPriorityQueue(stop <-chan struct{}) *PriorityQueue {
	return NewPriorityQueueWithClock(stop)
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

func addOrUpdateUnschedulablePod(p *PriorityQueue, pod Pod) {
	p.lock.w <- true
	defer func() { <-p.lock.w }()
	p.unschedulableQ.addOrUpdate(p.newPodInfo(pod))
}

type waitgroup struct {
	wait chan bool
	pool chan int
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
		q := NewPriorityQueue(nil)
		highPod := Pod("1")
		addOrUpdateUnschedulablePod(q, highPod)
		q.unschedulableQ.podInfoMap[GetPodFullName(highPod)].Timestamp = time.Now().Add(-1 * unschedulableQTimeInterval)
	}()
	<-wg.wait
}
