package lattice

import (
	"fmt"

	"github.com/benbjohnson/immutable"
)

// InfiniteMap is a wrapper around the base map.
type InfiniteMap[K any] struct {
	baseMap[K]
}

// newInfiniteMap initializes a fresh infinite map belonging to the given lattice.
func newInfiniteMap[K any](lat *InfiniteMapLattice[K]) InfiniteMap[K] {
	var mp *immutable.Map[K, Element]
	if lat.mapFactory == nil {
		mp = immutable.NewMap[K, Element](nil)
	} else {
		mp = lat.mapFactory()
	}

	return InfiniteMap[K]{
		baseMap[K]{element{lat}, mp},
	}
}

// MakeInfiniteMap generates an infinite map factory from bindings to infinite maps
// for members of the given infinite map lattice.
//
// func (elementFactory) InfiniteMap[K](lat Lattice) func(bindings map[interface{}]Element) InfiniteMap[K] {
func MakeInfiniteMap[K any](lat Lattice) func(bindings map[interface{}]Element) InfiniteMap[K] {
	switch lat := lat.(type) {
	case *InfiniteMapLattice[K]:
		return func(bindings map[interface{}]Element) InfiniteMap[K] {
			el := newInfiniteMap(lat)

			for x, y := range bindings {
				checkLatticeMatch(lat.rng, y.Lattice(), "map creation")
				el.mp = el.mp.Set(x.(K), y)
			}

			return el
		}
	case *Lifted:
		return MakeInfiniteMap[K](lat.Lattice)
	case *Dropped:
		return MakeInfiniteMap[K](lat.Lattice)
	default:
		panic("Attempted creating infinite map with a non-infinite map lattice")
	}
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (e1 InfiniteMap[K]) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes m = o.
func (e1 InfiniteMap[K]) eq(e2 Element) bool {
	if e2, ok := e2.(InfiniteMap[K]); ok {
		return e1.baseMap.eq(e2.baseMap)
	}

	return false
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (e1 InfiniteMap[K]) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes m ⊒ o.
func (e1 InfiniteMap[K]) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case InfiniteMap[K]:
		return e1.baseMap.geq(e2.baseMap)
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errPatternMatch(e2))
	}
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (e1 InfiniteMap[K]) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.leq(e2)
}

// leq computes m ⊑ o.
func (e1 InfiniteMap[K]) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case InfiniteMap[K]:
		return e1.baseMap.leq(e2.baseMap)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (e1 InfiniteMap[K]) Join(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes m ⊔ o.
func (e1 InfiniteMap[K]) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case InfiniteMap[K]:
		return e1.MonoJoin(e2)
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

// MonoJoin is a monomorphic variant of m ⊔ o for infinite maps.
func (e1 InfiniteMap[K]) MonoJoin(e2 InfiniteMap[K]) InfiniteMap[K] {
	e1.baseMap = e1.baseMap.MonoJoin(e2.baseMap)
	return e1
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (e1 InfiniteMap[K]) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes m ⊓ o.
func (e1 InfiniteMap[K]) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case InfiniteMap[K]:
		return e1.MonoMeet(e2)
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	default:
		panic(errInternal)
	}
}

// MonoMeet is a monomorphic variant of m ⊓ o for infinite maps.
func (e1 InfiniteMap[K]) MonoMeet(e2 InfiniteMap[K]) InfiniteMap[K] {
	e1.baseMap = e1.baseMap.MonoMeet(e2.baseMap)
	return e1
}

// Get retrieves the value bound at the given key.
func (e1 InfiniteMap[K]) Get(key K) Element {
	v, _ := e1.baseMap.Get(key)
	return v
}

// Update constructs a new map with updated bindings.
func (e1 InfiniteMap[K]) Update(x K, e2 Element) InfiniteMap[K] {
	mapLat := e1.Lattice().(*InfiniteMapLattice[K])
	checkLatticeMatchThunked(mapLat.rng, e2.Lattice(), func() string {
		return fmt.Sprintf("%s[ %v ↦  %s ]", e1, x, e2)
	})
	e1.baseMap = e1.baseMap.Update(x, e2)
	return e1
}
