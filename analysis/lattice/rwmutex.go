package lattice

//go:generate go run generate-product.go rwmutex

// RWMutexLattice represents the lattice of read-write mutex values.
type RWMutexLattice struct {
	ProductLattice
}

// rwmutexLattice is the singleton instantion of the read-write mutex lattice.
var rwmutexLattice = &RWMutexLattice{
	*latFact.Product(
		mutexLattice,
		flatIntLattice,
	),
}

// RWMutex safely converts to the read-write mutex lattice.
func (latticeFactory) RWMutex() *RWMutexLattice {
	return rwmutexLattice
}

// Top retrieves the ⊤ element of the read-write mutex lattice.
func (l RWMutexLattice) Top() Element {
	return RWMutex{
		element{rwmutexLattice},
		l.ProductLattice.Top().Product(),
	}
}

// Bot retrives the ⊥ element of the read-write mutex lattice.
func (l RWMutexLattice) Bot() Element {
	return RWMutex{
		element{rwmutexLattice},
		l.ProductLattice.Bot().Product(),
	}
}

// Eq checks for equality with another lattice.
func (l1 RWMutexLattice) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	case *RWMutexLattice:
		return true
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	}

	return false
}

func (RWMutexLattice) String() string {
	return colorize.Lattice("RWMutex")
}

// RWMutex generates an abstract read-write mutex value.
// By default the read-write mutex is UNLOCKED and has 0 read-locks.
func (elementFactory) RWMutex() RWMutex {
	return RWMutex{
		element{rwmutexLattice},
		elFact.Product(&rwmutexLattice.ProductLattice)(
			elFact.Flat(mutexLattice)(false),
			elFact.FlatInt(0),
		),
	}
}

// MaybeRLocked checks whether a read-write mutex may be read-locked,
// by checking whether the number of acquired read-locks is ⊤, or a known
// constant greater than 0.
func (w RWMutex) MaybeRLocked() bool {
	return w.RLocks().IsTop() || w.RLocks().FlatInt().IValue() > 0
}
