package lattice

// MutexLattice is the lattice of locks. It is represented as a flat finite lattice with elements
// LOCKED and UNLOCKED.
/*
	      ⊤
	     / \
	LOCKED UNLOCKED
	     \ /
	      ⊥
*/
type MutexLattice struct {
	FlatFiniteLattice
}

// mutexLattice is the singleton instantiation of the mutex lattice.
var mutexLattice = func() *MutexLattice {
	lat := &MutexLattice{*latFact.Flat(true, false)}
	lat.init(lat)
	return lat
}()

func (latticeFactory) Mutex() *MutexLattice {
	return mutexLattice
}

func (*MutexLattice) String() string {
	return colorize.Lattice("Mutex")
}

func (l1 *MutexLattice) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	case *MutexLattice:
		return true
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}
