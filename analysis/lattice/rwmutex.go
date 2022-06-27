package lattice

//go:generate go run generate-product.go RWMutex Status,FlatElement,Flat,Status RLocks,FlatElement,Flat,RLocks

type RWMutexLattice struct {
	ProductLattice
}

var rwmutexLattice = &RWMutexLattice{
	*latFact.Product(
		mutexLattice,
		flatIntLattice,
	),
}

func (latticeFactory) RWMutex() *RWMutexLattice {
	return rwmutexLattice
}

func (l RWMutexLattice) Top() Element {
	return RWMutex{
		element{rwmutexLattice},
		l.ProductLattice.Top().Product(),
	}
}

func (l RWMutexLattice) Bot() Element {
	return RWMutex{
		element{rwmutexLattice},
		l.ProductLattice.Bot().Product(),
	}
}

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

func (elementFactory) RWMutex() RWMutex {
	return RWMutex{
		element{rwmutexLattice},
		elFact.Product(&rwmutexLattice.ProductLattice)(
			elFact.Flat(mutexLattice)(false),
			elFact.FlatInt(0),
		),
	}
}

func (w RWMutex) MaybeRLocked() bool {
	return w.RLocks().IsTop() || w.RLocks().FlatInt().IValue() > 0
}
