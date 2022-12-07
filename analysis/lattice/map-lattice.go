package lattice

import (
	"fmt"

	i "github.com/cs-au-dk/goat/utils/indenter"

	"github.com/benbjohnson/immutable"
)

type latticeWithRange interface {
	RngBot() Element
}

type mapLatticeBase struct {
	lattice
	rng Lattice
}

func (m mapLatticeBase) RngBot() Element {
	return m.rng.Bot()
}

type MapLattice[K any] struct {
	mapLatticeBase
	top *Map[K]
	bot *Map[K]
	dom set
}

// Create a map lattice. Provide a range lattice,
// and a domain.
// func (latticeFactory) Map(rng Lattice, dom set) *MapLattice {
func MakeMapLattice[K any](rng Lattice, dom set) *MapLattice[K] {
	m := new(MapLattice[K])
	m.dom = make(set)
	for x := range dom {
		m.dom[x] = true
	}
	m.rng = rng
	return m
}

// func (latticeFactory) MapVariadic(rng Lattice, dom ...interface{}) *MapLattice[K] {
func MakeMapLatticeVariadic[K any](rng Lattice, dom ...interface{}) *MapLattice[K] {
	m := new(MapLattice[K])
	m.dom = make(set)
	for _, x := range dom {
		m.dom[x] = true
	}
	m.rng = rng
	return m
}

func (l *MapLattice[K]) Top() Element {
	if l.top == nil {
		l.top = new(Map[K])
		mp := immutable.NewMapBuilder[K, Element](nil)
		for x := range l.dom {
			mp.Set(x.(K), l.rng.Top())
		}
		*l.top = Map[K]{
			baseMap[K]{
				element{l},
				mp.Map(),
			},
		}
	}
	return *l.top
}

func (l *MapLattice[K]) Bot() Element {
	if l.bot == nil {
		l.bot = new(Map[K])
		*l.bot = newMap[K](l)
	}
	return *l.bot
}

func (l1 *MapLattice[K]) Eq(l2 Lattice) bool {
	// First try to get away with referential equality
	if l1 == l2 {
		return true
	}
	switch l2 := l2.(type) {
	case *MapLattice[K]:
		for x := range l1.dom {
			if contains, ok := l2.dom[x]; !contains || !ok {
				return false
			}
		}
		for x := range l2.dom {
			if contains, ok := l1.dom[x]; !contains || !ok {
				return false
			}
		}
		return l1.rng.Eq(l2.rng)
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (l *MapLattice[K]) String() string {
	strs := []fmt.Stringer{}

	for x := range l.dom {
		if xs, ok := x.(fmt.Stringer); ok {
			strs = append(strs, xs)
		}
	}

	return i.Indenter().Start("{").NestSep(",", strs...).End("} → " + l.rng.String())
}

func (l *MapLattice[K]) Range() Lattice {
	return l.rng
}

func (l *MapLattice[K]) Domain() set {
	return l.dom
}

// Specifies whether the map lattice domain includes x
func (e *MapLattice[K]) Contains(x interface{}) bool {
	_, ok := e.dom[x]
	return ok
}

func (e *MapLattice[K]) Map() *MapLattice[K] {
	return e
}
