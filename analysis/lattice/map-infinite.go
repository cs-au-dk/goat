package lattice

import (
	"fmt"

	"github.com/benbjohnson/immutable"
)

type InfiniteMap struct {
	MapBase
}

func newInfiniteMap(lat Lattice) InfiniteMap {
	return InfiniteMap{
		MapBase{
			element{lat},
			elementMap{immutable.NewMap(nil)},
		},
	}
}

func (elementFactory) InfiniteMap(lat Lattice) func(bindings map[interface{}]Element) InfiniteMap {
	switch lat := lat.(type) {
	case *InfiniteMapLattice:
		return func(bindings map[interface{}]Element) InfiniteMap {
			el := newInfiniteMap(lat)

			for x, y := range bindings {
				checkLatticeMatch(lat.rng, y.Lattice(), "map creation")
				el.mp = el.mp.set(x, y)
			}

			return el
		}
	case *Lifted:
		return elFact.InfiniteMap(lat.Lattice)
	case *Dropped:
		return elFact.InfiniteMap(lat.Lattice)
	default:
		panic("Attempted creating infinite map with a non-infinite map lattice")
	}
}

func (e InfiniteMap) Get(x interface{}) Element {
	if v, ok := e.mp.get(x); !ok {
		return e.lattice.InfiniteMap().rng.Bot()
	} else {
		return v
	}
}

func (e1 InfiniteMap) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 InfiniteMap) InfiniteMap() InfiniteMap {
	return e1
}

func (e1 InfiniteMap) eq(e2 Element) bool {
	switch e2 := e2.(type) {
	case InfiniteMap:
		if e1 == e2 {
			return true
		}
		compared := make(map[interface{}]bool)
		return e1.mp.forall(func(k interface{}, v1 Element) bool {
			compared[k] = true
			return v1.eq(e2.Get(k))
		}) && e2.mp.forall(func(k interface{}, v2 Element) bool {
			return compared[k] || e1.Get(k).eq(v2)
		})
	case *LiftedBot:
		return false
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

func (e1 InfiniteMap) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 InfiniteMap) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case InfiniteMap:
		if e1 == e2 {
			return true
		}
		compared := make(map[interface{}]bool)
		return e1.mp.forall(func(k interface{}, v1 Element) bool {
			compared[k] = true
			return v1.geq(e2.Get(k))
		}) && e2.mp.forall(func(k interface{}, v2 Element) bool {
			return compared[k] || e1.Get(k).geq(v2)
		})
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

func (e1 InfiniteMap) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.leq(e2)
}

func (e1 InfiniteMap) leq(e2 Element) bool {
	if e1 == e2 {
		return true
	}
	switch e2 := e2.(type) {
	case InfiniteMap:
		compared := make(map[interface{}]bool)
		return e1.mp.forall(func(k interface{}, v1 Element) bool {
			compared[k] = true
			return v1.leq(e2.Get(k))
		}) && e2.mp.forall(func(k interface{}, v2 Element) bool {
			return compared[k] || e1.Get(k).leq(v2)
		})
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

func (e1 InfiniteMap) Join(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 InfiniteMap) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case InfiniteMap:
		return e1.MonoJoin(e2)
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

func (e1 InfiniteMap) MonoJoin(e2 InfiniteMap) InfiniteMap {
	if e1 == e2 {
		return e1
	}
	e3 := newInfiniteMap(e1.lattice)
	joined := make(map[interface{}]bool)
	e1.mp.foreach(func(key interface{}, v1 Element) {
		joined[key] = true
		e3.mp = e3.mp.set(key, v1.join(e2.Get(key)))
	})
	e2.mp.foreach(func(key interface{}, v2 Element) {
		if joined[key] {
			return
		}
		e3.mp = e3.mp.set(key, v2.join(e1.Get(key)))
	})
	return e3
}

func (e1 InfiniteMap) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 InfiniteMap) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case InfiniteMap:
		return e1.MonoMeet(e2)
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	default:
		panic(errInternal)
	}
}

func (e1 InfiniteMap) MonoMeet(e2 InfiniteMap) InfiniteMap {
	if e1 == e2 {
		return e1
	}
	e3 := newInfiniteMap(e1.lattice)
	met := make(map[interface{}]bool)
	e1.mp.foreach(func(key interface{}, v1 Element) {
		met[key] = true
		e3.mp = e3.mp.set(key, v1.meet(e2.Get(key)))
	})
	e2.mp.foreach(func(key interface{}, v2 Element) {
		if met[key] {
			return
		}
		e3.mp = e3.mp.set(key, v2.meet(e1.Get(key)))
	})
	return e3
}

func (e1 InfiniteMap) Update(x interface{}, e2 Element) InfiniteMap {
	mapLat := e1.Lattice().InfiniteMap()
	checkLatticeMatchThunked(mapLat.rng, e2.Lattice(), func() string {
		return fmt.Sprintf("%s[ %s ↦  %s ]", e1, x, e2)
	})
	e1.mp = e1.mp.set(x, e2)
	return e1
}

func (e InfiniteMap) ForEach(do func(interface{}, Element)) {
	e.mp.foreach(do)
}
