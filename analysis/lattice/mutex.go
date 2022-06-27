package lattice

type MutexLattice struct {
	FlatFiniteLattice
}

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
