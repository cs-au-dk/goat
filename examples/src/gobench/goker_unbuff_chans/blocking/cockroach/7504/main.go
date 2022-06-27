/*
 * Project: cockroach
 * Issue or PR  : https://github.com/cockroachdb/cockroach/pull/7504
 * Buggy version: bc963b438cdc3e0ad058a5282358e5aee0595e17
 * fix commit-id: cab761b9f5ee5dee1448bc5d6b1d9f5a0ff0bad5
 * Flaky: 1/100
 * Description: There are locking leaseState, tableNameCache in Release(), but
 * tableNameCache,LeaseState in AcquireByName.  It is AB and BA deadlock.
 */
package main

const tableSize = 1

func MakeCacheKey(lease *LeaseState) int {
	return lease.id
}

type LeaseState struct {
	mu chan bool // LockA
	id int
}
type LeaseSet struct {
	data []*LeaseState
}

func (l *LeaseSet) insert(s *LeaseState) {
	s.id = len(l.data)
	l.data = append(l.data, s)
}
func (l *LeaseSet) find(id int) *LeaseState {
	return l.data[id]
}
func (l *LeaseSet) remove(s *LeaseState) {
	for i := 0; i < len(l.data); i++ {
		if s == l.data[i] {
			l.data = append(l.data[:i], l.data[i+1:]...)
			break
		}
	}
}

type tableState struct {
	tableNameCache *tableNameCache
	mu             chan bool
	active         *LeaseSet
}

func (t *tableState) release(lease *LeaseState) {
	t.mu <- true
	defer func() {
		<-t.mu
	}()

	s := t.active.find(MakeCacheKey(lease))
	s.mu <- true // LockA acquire
	defer func() {
		<-s.mu
	}() // LockA release

	t.removeLease(s)
}
func (t *tableState) removeLease(lease *LeaseState) {
	t.active.remove(lease)
	t.tableNameCache.remove(lease) // LockA acquire/release
}

type tableNameCache struct {
	mu     chan bool // LockB
	tables map[int]*LeaseState
}

func (c *tableNameCache) get(id int) {
	c.mu <- true
	defer func() {
		<-c.mu
	}()
	lease, ok := c.tables[id]
	if !ok {
		return
	}
	if lease == nil {
		panic("nil lease in name cache")
	}
	//+time.Sleep(time.Second)
	lease.mu <- true // LockB acquire
	defer func() {
		<-lease.mu
	}()
	// LockB release
	// LockA release
}

func (c *tableNameCache) remove(lease *LeaseState) {
	c.mu <- true // LockA acquire
	defer func() {
		<-c.mu
	}()
	key := MakeCacheKey(lease)
	existing, ok := c.tables[key]
	if !ok {
		return
	}
	if existing == lease {
		delete(c.tables, key)
	}
	// LockA release
}

type LeaseManager struct {
	_          [64]byte
	mu         chan bool
	tableNames *tableNameCache
	tables     map[int]*tableState
}

func (m *LeaseManager) AcquireByName(id int) {
	m.tableNames.get(id)
}

func (m *LeaseManager) findTableState(lease *LeaseState) *tableState {
	existing, ok := m.tables[0]
	if !ok {
		return nil
	}
	return existing
}

func (m *LeaseManager) Release(lease *LeaseState) {
	t := m.findTableState(lease)
	t.release(lease)
}
func NewLeaseManager(tname *tableNameCache, ts *tableState) *LeaseManager {
	mgr := &LeaseManager{
		tableNames: tname,
		tables:     make(map[int]*tableState),
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
	mgr.tables[0] = ts
	return mgr
}
func NewLeaseSet(n int) *LeaseSet {
	lset := &LeaseSet{}
	for i := 0; i < n; i++ {
		lease := new(LeaseState)
		lease.mu = func() (lock chan bool) {
			lock = make(chan bool)
			go func() {
				for {
					<-lock
					lock <- false
				}
			}()
			return
		}()
		lset.data = append(lset.data, lease)
	}
	return lset
}

func main() {
	leaseNum := 2
	lset := NewLeaseSet(leaseNum)

	nc := &tableNameCache{
		tables: make(map[int]*LeaseState),
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
	for i := 0; i < leaseNum; i++ {
		nc.tables[i] = lset.find(i)
	}

	ts := &tableState{
		tableNameCache: nc,
		active:         lset,
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

	mgr := NewLeaseManager(nc, ts)

	// G1
	go func() {
		// lock AB
		mgr.AcquireByName(0)
	}()

	// G2
	go func() {
		// lock BA
		mgr.Release(lset.find(0))
	}()
}
