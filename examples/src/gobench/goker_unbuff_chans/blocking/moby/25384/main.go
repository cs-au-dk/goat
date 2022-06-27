/*
 * Project: moby
 * Issue or PR  : https://github.com/moby/moby/pull/25384
 * Buggy version: 58befe3081726ef74ea09198cd9488fb42c51f51
 * fix commit-id: 42360d164b9f25fb4b150ef066fcf57fa39559a7
 * Flaky: 100/100
 * Description:
 *   When n=1 (len(pm.plugins)), the location of group.Wait() doesn’t matter.
 * When n is larger than 1, group.Wait() is invoked in each iteration. Whenever
 * group.Wait() is invoked, it waits for group.Done() to be executed n times.
 * However, group.Done() is only executed once in one iteration.
 */
package main

type plugin struct{}

type Manager struct {
	plugins []*plugin
}

type waitgroup struct {
	pool chan int
	wait chan bool
}

func (pm *Manager) init() {
	var group = func() (wg waitgroup) {
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
	group.pool <- len(pm.plugins)
	for _, p := range pm.plugins {
		go func(p *plugin) {
			defer func() { <-group.pool }()
		}(p)
		<-group.wait // Block here
	}
}
func main() {
	p1 := &plugin{}
	p2 := &plugin{}
	pm := &Manager{
		plugins: []*plugin{p1, p2},
	}
	go pm.init()
}
