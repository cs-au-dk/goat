package lattice

import (
	i "Goat/utils/indenter"
	"fmt"
	"sort"

	"github.com/benbjohnson/immutable"
)

type MapBase struct {
	element
	mp elementMap
}

type Map struct {
	MapBase
}

func newMap(lat Lattice) Map {
	// Should always be safe
	mapLat := lat.Map()
	mp := immutable.NewMapBuilder(nil)
	for x := range mapLat.dom {
		mp.Set(x, mapLat.rng.Bot())
	}
	return Map{
		MapBase{
			element{mapLat},
			elementMap{mp.Map()},
		},
	}
}

func (elementFactory) Map(lat Lattice) func(bindings map[interface{}]Element) Map {
	switch lat := lat.(type) {
	case *MapLattice:
		return func(bindings map[interface{}]Element) Map {
			el := newMap(lat)

			for x, y := range bindings {
				if !y.Lattice().Eq(lat.rng) {
					panic(errUnsupportedTypeConversion)
				}
				el.mp = el.mp.set(x, y)
			}

			return el
		}
	case *Lifted:
		return elFact.Map(lat.Lattice)
	case *Dropped:
		return elFact.Map(lat.Lattice)
	default:
		panic("Attempted creating map with a non-map lattice")
	}
}

func (e MapBase) String() string {
	strs := []string{}
	e.mp.foreach(func(x interface{}, y Element) {
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

func (m MapBase) Height() (h int) {
	m.mp.foreach(func(key interface{}, e Element) {
		var getRngLat func(Lattice) Lattice
		getRngLat = func(l Lattice) Lattice {
			switch l := l.(type) {
			case *MapLattice:
				return l.rng
			case *InfiniteMapLattice:
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

func (e Map) Map() Map {
	return e
}

func (e1 Map) Geq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 Map) geq(e2 Element) (result bool) {
	switch e2 := e2.(type) {
	case Map:
		return e1.mp.forall(func(k interface{}, e Element) bool {
			return e.geq(e2.Get(k))
		})
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errPatternMatch(e2))
	}
}

func (e1 Map) Leq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 Map) leq(e2 Element) (result bool) {
	switch e2 := e2.(type) {
	case Map:
		return e1.mp.forall(func(k interface{}, e Element) bool {
			return e.leq(e2.Get(k))
		})
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

func (e1 Map) Eq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 Map) eq(e2 Element) bool {
	if e1 == e2 {
		return true
	}
	return e1.geq(e2) && e1.leq(e2)
}

func (e1 Map) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 Map) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case Map:
		return e1.MonoJoin(e2)
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

func (e1 Map) MonoJoin(e2 Map) Map {
	e3 := newMap(e1.lattice)
	e1.mp.foreach(func(key interface{}, e Element) {
		e3.mp = e3.mp.set(
			key,
			e1.Get(key).join(e2.Get(key)),
		)
	})
	return e3
}

func (e1 Map) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 Map) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case Map:
		return e1.MonoMeet(e2)
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	default:
		panic(errInternal)
	}
}

func (e1 Map) MonoMeet(e2 Map) Map {
	e3 := newMap(e1.lattice)
	e1.mp.foreach(func(key interface{}, e Element) {
		e3.mp = e3.mp.set(
			key,
			e1.Get(key).meet(e2.Get(key)),
		)
	})
	return e3

}

func (e Map) Get(x interface{}) Element {
	mapLat := e.Lattice().Map()
	if _, ok := mapLat.dom[x]; !ok {
		panic(fmt.Sprintf("%s is not part of the domain of map lattice:\n%s", x, mapLat))
	}
	return e.mp.getUnsafe(x)
}

func (e1 Map) Update(x interface{}, e2 Element) Map {
	mapLat := e1.Lattice().Map()
	if _, ok := mapLat.dom[x]; !ok {
		panic(fmt.Sprintf("%s is not part of the domain of map lattice:\n%s", x, mapLat))
	}
	checkLatticeMatchThunked(mapLat.rng, e2.Lattice(), func() string {
		return fmt.Sprintf("%s[ %s ↦  %s ]", e1, x, e2)
	})
	return e1.update(x, e2)
}

func (e1 Map) update(x interface{}, e2 Element) Map {
	e1.mp = e1.mp.set(x, e2)
	return e1
}

func (e Map) ForEach(f func(interface{}, Element)) {
	e.mp.foreach(f)
}

func (e Map) ForAll(f func(interface{}, Element) bool) bool {
	return e.mp.forall(f)
}
