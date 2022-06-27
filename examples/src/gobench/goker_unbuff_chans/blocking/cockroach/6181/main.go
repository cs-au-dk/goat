/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/6181
 * Buggy version: c0a232b5521565904b851699853bdbd0c670cf1e
 * fix commit-id: d5814e4886a776bf7789b3c51b31f5206480d184
 * Flaky: 57/100
 */
package main

type testDescriptorDB struct {
	cache *rangeDescriptorCache
}

func initTestDescriptorDB() *testDescriptorDB {
	return &testDescriptorDB{&rangeDescriptorCache{}}
}

type rangeDescriptorCache struct {
	rangeCacheMu struct {
		W chan bool
		R chan bool
	}
}

func (rdc *rangeDescriptorCache) LookupRangeDescriptor() {
	rdc.rangeCacheMu.R <- true
	println("lookup range descriptor: %s", rdc)
	<-rdc.rangeCacheMu.R
	rdc.rangeCacheMu.W <- true
	<-rdc.rangeCacheMu.W
}

func (rdc *rangeDescriptorCache) String() string {
	rdc.rangeCacheMu.R <- true
	defer func() {
		<-rdc.rangeCacheMu.R
	}()
	return rdc.stringLocked()
}

func (rdc *rangeDescriptorCache) stringLocked() string {
	return "something here"
}

func doLookupWithToken(rc *rangeDescriptorCache) {
	rc.LookupRangeDescriptor()
}

func testRangeCacheCoalescedRquests() {
	db := initTestDescriptorDB()
	pauseLookupResumeAndAssert := func() {
		var wg = func() (wg struct {
			Pool chan int
			Wait chan bool
		}) {
			wg = struct {
				Pool chan int
				Wait chan bool
			}{
				Pool: make(chan int),
				Wait: make(chan bool),
			}

			go func() {
				count := 0

				for {
					select {
					// The WaitGroup may wait so long as the count is 0.
					case wg.Wait <- true:
					// The first pooled goroutine will prompt the WaitGroup to wait
					// and disregard all sends on Wait until all pooled goroutines unblock.
					case x := <-wg.Pool:
						count += x
						// TODO: Simulate counter dropping below 0 panics.
						for count > 0 {
							select {
							case x := <-wg.Pool:
								count += x
							// Caller should receive on wg.Pool to decrement counter
							case wg.Pool <- 0:
								count--
							}
						}
					}
				}
			}()

			return
		}()
		for i := 0; i < 3; i++ {
			wg.Pool <- 1
			go func() { // G2,G3,...
				doLookupWithToken(db.cache)
				<-wg.Pool
			}()
		}
		<-wg.Wait
	}
	pauseLookupResumeAndAssert()
}

/// G1 									G2							G3					...
/// testRangeCacheCoalescedRquests()
/// initTestDescriptorDB()
/// pauseLookupResumeAndAssert()
/// return
/// 									doLookupWithToken()
///																 	doLookupWithToken()
///										rc.LookupRangeDescriptor()
///																	rc.LookupRangeDescriptor()
///										rdc.rangeCacheMu.RLock()
///										rdc.String()
///																	rdc.rangeCacheMu.RLock()
///																	fmt.Printf()
///																	rdc.rangeCacheMu.RUnlock()
///																	rdc.rangeCacheMu.Lock()
///										rdc.rangeCacheMu.RLock()
/// -------------------------------------G2,G3,... deadlock--------------------------------------

func main() {
	go testRangeCacheCoalescedRquests() // G1
}
