package lattice

import (
	"fmt"

	"github.com/benbjohnson/immutable"
)

// Map is a member of the map lattice with keys belonging to `K`.
type Map[K any] struct {
	baseMap[K]
}

// newMap creates a new map for the given map lattice.
func newMap[K any](lat Lattice) Map[K] {
	// Should always be safe
	mapLat := lat.(*MapLattice[K])
	mp := immutable.NewMapBuilder[K, Element](nil)
	for x := range mapLat.dom {
		mp.Set(x.(K), mapLat.rng.Bot())
	}
	return Map[K]{
		baseMap[K]{
			element{mapLat},
			mp.Map(),
		},
	}
}

// MakeMap generates a map factory converting a set of bindings to
// members of the given map lattice.
//
// func (elementFactory) Map(lat Lattice) func(bindings map[interface{}]Element) Map[K] {
func MakeMap[K any](lat Lattice) func(bindings map[any]Element) Map[K] {
	switch lat := lat.(type) {
	case *MapLattice[K]:
		return func(bindings map[interface{}]Element) Map[K] {
			el := newMap[K](lat)

			for x, y := range bindings {
				if !y.Lattice().Eq(lat.rng) {
					panic(errUnsupportedTypeConversion)
				}
				el.mp = el.mp.Set(x.(K), y)
			}

			return el
		}
	case *Lifted:
		return MakeMap[K](lat.Lattice)
	case *Dropped:
		return MakeMap[K](lat.Lattice)
	default:
		panic("Attempted creating map with a non-map lattice")
	}
}

/*
type MapBase[K any] struct {
	element
	mp elementMap[K]
}

func (e MapBase[K]) String() string {
	strs := []string{}
	e.mp.foreach(func(x K, y Element) {
		if !y.eq(y.Lattice().Bot()) {
			strs = append(strs, fmt.Sprintf("%v ↦  %s", x, y))
		}
	})
	if len(strs) == 0 {
		return "[]"
	}
	sort.Slice(strs, func(i int, j int) bool {
		return strs[i] < strs[j]
	})
	return i.Indenter().Start("[").
		NestStringsSep(",", strs...).
		End("]")
}

func (m MapBase[K]) Height() (h int) {
	m.mp.foreach(func(key K, e Element) {
		getRngLat := func(l Lattice) Lattice {
			switch l := l.(type) {
			case *MapLattice[K]:
				return l.rng
			case *InfiniteMapLattice[K]:
				return l.rng
			}
			return nil
		}
		elat := getRngLat(m.lattice)

		switch e := e.(type) {
		case *LiftedBot:
			h += elat.Preheight() - (e.Index() + 1)
		default:
			h += elat.Preheight() + e.Height()
		}
	})

	return
}

func (m MapBase[K]) Size() int {
	return m.mp.Len()
}
*/

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (e1 Map[K]) Geq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes m ⊒ o.
func (e1 Map[K]) geq(e2 Element) (result bool) {
	switch e2 := e2.(type) {
	case Map[K]:
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
func (e1 Map[K]) Leq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes m ⊑ o.
func (e1 Map[K]) leq(e2 Element) (result bool) {
	switch e2 := e2.(type) {
	case Map[K]:
		return e1.baseMap.leq(e2.baseMap)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (e1 Map[K]) Eq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes m = o.
func (e1 Map[K]) eq(e2 Element) bool {
	if e2, ok := e2.(Map[K]); ok {
		return e1.baseMap.eq(e2.baseMap)
	}

	return false
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (e1 Map[K]) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes m ⊔ o.
func (e1 Map[K]) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case Map[K]:
		return e1.MonoJoin(e2)
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

// MonoJoin is a monomorphic variant of m ⊔ o for maps.
func (e1 Map[K]) MonoJoin(e2 Map[K]) Map[K] {
	e1.baseMap = e1.baseMap.MonoJoin(e2.baseMap)
	return e1
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (e1 Map[K]) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes m ⊓ o.
func (e1 Map[K]) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case Map[K]:
		return e1.MonoMeet(e2)
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	default:
		panic(errInternal)
	}
}

// MonoMeet is a monomorphic variant of m ⊓ o for maps.
func (e1 Map[K]) MonoMeet(e2 Map[K]) Map[K] {
	e1.baseMap = e1.baseMap.MonoMeet(e2.baseMap)
	return e1
}

// Get retrieves the value bound at the given key.
func (e Map[K]) Get(x K) Element {
	mapLat := e.Lattice().(*MapLattice[K])
	if _, ok := mapLat.dom[x]; !ok {
		panic(fmt.Sprintf("%v is not part of the domain of map lattice:\n%s", x, mapLat))
	}
	return e.GetUnsafe(x)
}

// Update returns a map with an updated binding for the given key.
// Performs dynamic lattice type checking.
func (e1 Map[K]) Update(x K, e2 Element) Map[K] {
	mapLat := e1.Lattice().(*MapLattice[K])
	if _, ok := mapLat.dom[x]; !ok {
		panic(fmt.Sprintf("%v is not part of the domain of map lattice:\n%s", x, mapLat))
	}
	checkLatticeMatchThunked(mapLat.rng, e2.Lattice(), func() string {
		return fmt.Sprintf("%s[ %v ↦  %s ]", e1, x, e2)
	})
	return e1.update(x, e2)
}

// update returns a map with an updated binding for the given key.
func (e1 Map[K]) update(x K, e2 Element) Map[K] {
	e1.baseMap = e1.baseMap.Update(x, e2)
	return e1
}
