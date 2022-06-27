package main

type Address int

type rwmutex struct {
	w chan bool
	r chan bool
}

type Mapping struct {
	mut rwmutex

	extAddresses map[string]Address
}

func (m *Mapping) clearAddresses() {
	m.mut.w <- true // First locking
	var removed []Address
	for id, addr := range m.extAddresses {
		removed = append(removed, addr)
		delete(m.extAddresses, id)
	}
	if len(removed) > 0 {
		m.notify(nil, removed)
	}
	<-m.mut.w
}

func (m *Mapping) notify(added, remove []Address) {
	m.mut.r <- true // Second locking
	<-m.mut.r
}

type Service struct {
	mut rwmutex

	mappings []*Mapping
}

func (s *Service) NewMapping() *Mapping {
	mapping := &Mapping{
		mut: func() (lock rwmutex) {
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
		extAddresses: make(map[string]Address),
	}
	s.mut.w <- true
	s.mappings = append(s.mappings, mapping)
	<-s.mut.w
	return mapping
}

func (s *Service) RemoveMapping(mapping *Mapping) {
	s.mut.w <- true
	defer func() { <-s.mut.w }()
	for _, existing := range s.mappings {
		if existing == mapping {
			mapping.clearAddresses()
		}
	}
}

func NewService() *Service {
	return &Service{
		mut: func() (lock rwmutex) {
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

func main() {
	natSvc := NewService()
	m := natSvc.NewMapping()
	m.extAddresses["test"] = 0

	natSvc.RemoveMapping(m)
}
