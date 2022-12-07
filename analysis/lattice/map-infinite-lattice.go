package lattice

import "github.com/benbjohnson/immutable"

type InfiniteMapLattice[K any] struct {
	mapLatticeBase
	dom        string
	mapFactory func() *immutable.Map[K, Element]
	// top *InfiniteMap
	bot *InfiniteMap[K]
}

// The domain is infinite so cannot be represented
// except as a name.
//func (latticeFactory) InfiniteMap(rng Lattice, dom string) *InfiniteMapLattice {
func MakeInfiniteMapLattice[K any](rng Lattice, dom string) *InfiniteMapLattice[K] {
	m := new(InfiniteMapLattice[K])
	m.rng = rng
	m.dom = dom
	return m
}

//func (l latticeFactory) InfiniteMapWFactory(
func MakeInfiniteMapLatticeWFactory[K any](
	rng Lattice,
	dom string,
	factory func() *immutable.Map[K, Element],
) *InfiniteMapLattice[K] {
	m := MakeInfiniteMapLattice[K](rng, dom)
	m.mapFactory = factory
	return m
}

func (l *InfiniteMapLattice[K]) Top() Element {
	panic(errUnsupportedOperation)
}

func (l *InfiniteMapLattice[K]) Bot() Element {
	if l.bot == nil {
		l.bot = new(InfiniteMap[K])
		*l.bot = newInfiniteMap(l)
	}
	return *l.bot
}

func (l1 *InfiniteMapLattice[K]) Eq(l2 Lattice) bool {
	if l1 == l2 {
		return true
	}
	switch l2 := l2.(type) {
	case *InfiniteMapLattice[K]:
		return l1.dom == l2.dom && l1.rng.Eq(l2.rng)
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (l *InfiniteMapLattice[K]) String() string {
	return colorize.Lattice(l.dom) + " → " + l.rng.String()
}

func (l *InfiniteMapLattice[K]) InfiniteMap() *InfiniteMapLattice[K] {
	return l
}
