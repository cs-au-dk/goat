package lattice

type InfiniteMapLattice struct {
	mapLatticeBase
	dom string
	// top *InfiniteMap
	bot *InfiniteMap
}

// The domain is infinite so cannot be represented
// except as a name.
func (latticeFactory) InfiniteMap(rng Lattice, dom string) *InfiniteMapLattice {
	m := new(InfiniteMapLattice)
	m.rng = rng
	m.dom = dom
	return m
}

func (l *InfiniteMapLattice) Top() Element {
	panic(errUnsupportedOperation)
}

func (l *InfiniteMapLattice) Bot() Element {
	if l.bot == nil {
		l.bot = new(InfiniteMap)
		*l.bot = newInfiniteMap(l)
	}
	return *l.bot
}

func (l1 *InfiniteMapLattice) Eq(l2 Lattice) bool {
	if l1 == l2 {
		return true
	}
	switch l2 := l2.(type) {
	case *InfiniteMapLattice:
		return l1.dom == l2.dom && l1.rng.Eq(l2.rng)
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (l *InfiniteMapLattice) String() string {
	return colorize.Lattice(l.dom) + " → " + l.rng.String()
}

func (l *InfiniteMapLattice) InfiniteMap() *InfiniteMapLattice {
	return l
}
