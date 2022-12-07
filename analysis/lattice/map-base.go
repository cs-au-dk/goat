package lattice

import (
	"fmt"
	"log"

	i "github.com/cs-au-dk/goat/utils/indenter"

	"github.com/benbjohnson/immutable"
)

type baseMap[K any] struct {
	element
	mp *immutable.Map[K, Element]
}

func (m baseMap[K]) Size() int {
	return m.mp.Len()
}

func (m baseMap[K]) Contains(key K) bool {
	_, found := m.mp.Get(key)
	return found
}

// Perform a lookup in the map. The returned boolean indicates if the given key was found.
func (m baseMap[K]) Get(k K) (Element, bool) {
	v, found := m.mp.Get(k)
	if !found {
		return m.Lattice().(latticeWithRange).RngBot(), false
	}

	return v, true
}

func (m baseMap[K]) GetOrDefault(k K, def Element) Element {
	if v, found := m.Get(k); found {
		return v
	} else {
		return def
	}
}

func (m baseMap[K]) GetUnsafe(k K) Element {
	if v, found := m.Get(k); found {
		return v
	}

	log.Fatalf("GetUnsafe: %v not found", k)
	panic("Unreachable")
}

func (m baseMap[K]) Find(f func(k K, e Element) bool) (K, Element, bool) {
	for iter := m.mp.Iterator(); !iter.Done(); {
		k, e, _ := iter.Next()

		if f(k, e) {
			return k, e, true
		}
	}

	var zk K
	return zk, nil, false
}

func (m baseMap[K]) Update(loc K, elem Element) baseMap[K] {
	m.mp = m.mp.Set(loc, elem)
	return m
}

func (m baseMap[K]) WeakUpdate(key K, elem Element) baseMap[K] {
	if prev, found := m.Get(key); found {
		return m.Update(key, prev.Join(elem))
	} else {
		return m.Update(key, elem)
	}
}

func (m baseMap[K]) Remove(key K) baseMap[K] {
	m.mp = m.mp.Delete(key)
	return m
}

func (a baseMap[K]) ForEach(do func(K, Element)) {
	for iter := a.mp.Iterator(); !iter.Done(); {
		k, v, _ := iter.Next()
		do(k, v)
	}
}

func (m baseMap[K]) ForAll(pred func(K, Element) bool) bool {
	for iter := m.mp.Iterator(); !iter.Done(); {
		k, v, _ := iter.Next()
		if !pred(k, v) {
			return false
		}
	}
	return true
}

func (m baseMap[K]) String() string {
	return m.StringFiltered(func(K) bool { return true })
}

func (m baseMap[K]) StringFiltered(filter func(K) bool) string {
	name := m.Lattice().String()
	length := m.Size()
	if length == 0 {
		return name + ": Empty"
	}

	buf := make([]func() string, 0, length)

	itr := m.mp.Iterator()
	for !itr.Done() {
		k, v, _ := itr.Next()
		if filter(k) {
			buf = append(buf, func() string {
				return fmt.Sprintf("%v ↦ %s", k, v)
			})
		}
	}

	// sort.Slice(buf, func(i, j int) bool {
	// 	return buf[i]() < buf[j]()
	// })
	return i.Indenter().Start(name + ": {").NestThunked(buf...).End("}")
}

// Monomorphic join. Returns BaseMap, skipping a type-conversion.
func (m baseMap[K]) MonoJoin(o baseMap[K]) baseMap[K] {
	if m.Size() == 0 {
		return o
	} else if o.Size() == 0 {
		return m
	} else if m.Size() < o.Size() {
		m, o = o, m
	} else if m.mp == o.mp {
		return m
	}

	for itr := o.mp.Iterator(); !itr.Done(); {
		k, v, _ := itr.Next()

		my_v, found := m.Get(k)
		if found {
			if !v.Eq(my_v) {
				m = m.Update(k, v.Join(my_v))
			}
		} else {
			m = m.Update(k, v)
		}
	}

	return m
}

func (m baseMap[K]) Join(o Element) Element {
	checkLatticeMatch(m.lattice, o.Lattice(), "⊔")
	return m.join(o)
}

func (m baseMap[K]) join(o Element) Element {
	switch o := o.(type) {
	case baseMap[K]:
		return m.MonoJoin(o)
	case *DroppedTop:
		return o
	case *LiftedBot:
		return m
	default:
		panic(errInternal)
	}
}

func (m baseMap[K]) Eq(o Element) bool {
	checkLatticeMatch(m.lattice, o.Lattice(), "=")
	return m.eq(o)
}

func (e1 baseMap[K]) eq(e2 Element) bool {
	switch e2 := e2.(type) {
	case baseMap[K]:
		if e1.mp == e2.mp {
			return true
		} else if e1.Size() != e2.Size() {
			return false
		}

		for itr := e1.mp.Iterator(); !itr.Done(); {
			k, v1, _ := itr.Next()

			v2, found := e2.Get(k)
			if !found || !v1.eq(v2) {
				return false
			}
		}

		return true
	default:
		return false
	}
}

func (e1 baseMap[K]) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 baseMap[K]) geq(e2 Element) bool {
	return e2.leq(e1) // OBS
}

func (e1 baseMap[K]) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 baseMap[K]) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case baseMap[K]:
		if e1.mp == e2.mp {
			return true
		} else if e1.Size() > e2.Size() {
			return false
		}

		for i := e1.mp.Iterator(); !i.Done(); {
			k, v1, _ := i.Next()
			v2, found := e2.mp.Get(k)
			if !found || !v1.leq(v2) {
				return false
			}
		}
		return true
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

func (m baseMap[K]) MonoMeet(o baseMap[K]) baseMap[K] {
	panic(errUnsupportedOperation)
}

func (m baseMap[K]) Meet(o Element) Element {
	panic(errUnsupportedOperation)
}

func (m baseMap[K]) meet(o Element) Element {
	panic(errUnsupportedOperation)
}
