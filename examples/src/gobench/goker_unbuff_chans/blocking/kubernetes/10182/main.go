/*
 * Project: kubernetes
 * Issue or PR  : https://github.com/kubernetes/kubernetes/pull/10182
 * Buggy version: 4b990d128a17eea9058d28a3b3688ab8abafbd94
 * fix commit-id: 64ad3e17ad15cd0f9a4fd86706eec1c572033254
 * Flaky: 15/100
 * Description:
 *   This is a lock-channel bug. goroutine 1 is blocked on a lock
 * held by goroutine 3, while goroutine 3 is blocked on sending
 * message to ch, which is read by goroutine 1.
 */
package main

type rwmutex struct {
	w chan bool
	r chan bool
}

type statusManager struct {
	podStatusesLock  rwmutex
	podStatusChannel chan bool
}

func (s *statusManager) Start() {
	go func() {
		for i := 0; i < 2; i++ {
			s.syncBatch()
		}
	}()
}

func (s *statusManager) syncBatch() {
	<-s.podStatusChannel
	s.DeletePodStatus()
}

func (s *statusManager) DeletePodStatus() {
	s.podStatusesLock.w <- true
	defer func() {
		<-s.podStatusesLock.w
	}()
}

func (s *statusManager) SetPodStatus() {
	s.podStatusesLock.w <- true
	defer func() {
		<-s.podStatusesLock.w
	}()
	s.podStatusChannel <- true
}

func NewStatusManager() *statusManager {
	return &statusManager{
		podStatusesLock: func() (lock rwmutex) {
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
		podStatusChannel: make(chan bool),
	}
}

/// G1 						G2							G3
/// s.Start()
/// s.syncBatch()
/// 						s.SetPodStatus()
/// <-s.podStatusChannel
/// 						s.podStatusesLock.Lock()
/// 						s.podStatusChannel <- true
/// 						s.podStatusesLock.Unlock()
/// 						return
/// s.DeletePodStatus()
/// 													s.podStatusesLock.Lock()
/// 													s.podStatusChannel <- true
/// s.podStatusesLock.Lock()
/// -----------------------------G1,G3 deadlock----------------------------
func main() {
	s := NewStatusManager()
	go s.Start()
	go s.SetPodStatus() // G2
	go s.SetPodStatus() // G3
}
