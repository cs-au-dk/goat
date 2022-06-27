/*
 * Project: etcd
 * Issue or PR  : https://github.com/coreos/etcd/pull/7902
 * Buggy version: dfdaf082c51ba14861267f632f6af795a27eb4ef
 * fix commit-id: 87d99fe0387ee1df1cf1811d88d37331939ef4ae
 * Flaky: 100/100
 * Description:
 *   At least two goroutines are needed to trigger this bug,
 * one is leader and the other is follower. Both the leader
 * and the follower execute the code above. If the follower
 * acquires mu.Lock() firstly and enter rc.release(), it will
 * be blocked at <- rcNextc (nextc). Only the leader can execute
 * close(nextc) to unblock the follower inside rc.release().
 * However, in order to invoke rc.release(), the leader needs
 * to acquires mu.Lock().
 *   The fix is to remove the lock and unlock around rc.release().
 */
package main

import (
	"time"
)

type roundClient struct {
	progress int
	acquire  func()
	validate func()
	release  func()
}

func runElectionFunc() {
	rcs := make([]roundClient, 3)
	nextc := make(chan bool)
	for i := range rcs {
		var rcNextc chan bool
		setRcNextc := func() {
			rcNextc = nextc
		}
		rcs[i].acquire = func() {}
		rcs[i].validate = func() {
			setRcNextc()
		}
		rcs[i].release = func() {
			if i == 0 { // Assume the first roundClient is the leader
				close(nextc)
				nextc = make(chan bool)
			}
			<-rcNextc // Followers is blocking here
		}
	}
	doRounds(rcs, 100)
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func doRounds(rcs []roundClient, rounds int) {
	var mu = func() (lock chan bool) {
		lock = make(chan bool)
		go func() {
			for {
				<-lock
				lock <- false
			}
		}()
		return
	}()
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
	wg.pool <- len(rcs)
	for i := range rcs {
		go func(rc *roundClient) { // G2,G3
			defer func() {
				<-wg.pool
			}()
			for rc.progress < rounds || rounds <= 0 {
				rc.acquire()
				mu <- true
				rc.validate()
				<-mu
				time.Sleep(10 * time.Millisecond)
				rc.progress++
				mu <- true // Leader is blocking here
				rc.release()
				<-mu
			}
		}(&rcs[i])
	}
	<-wg.wait
}

///
/// G1						G2 (leader)					G3 (follower)
/// runElectionFunc()
/// doRounds()
/// wg.Wait()
/// 						...
/// 						mu.Lock()
/// 						rc.validate()
/// 						rcNextc = nextc
/// 						mu.Unlock()					...
/// 													mu.Lock()
/// 													rc.validate()
/// 													mu.Unlock()
/// 													mu.Lock()
/// 													rc.release()
/// 													<-rcNextc
/// 						mu.Lock()
/// -------------------------G1,G2,G3 deadlock--------------------------
///
func main() {
	go runElectionFunc() // G1
}
