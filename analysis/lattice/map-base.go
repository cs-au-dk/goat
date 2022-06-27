package lattice

import (
	i "Goat/utils/indenter"
	"fmt"
	"log"

	"github.com/benbjohnson/immutable"
)

type baseMap struct {
	element
	mp *immutable.Map
}

func (m baseMap) Size() int {
	return m.mp.Len()
}

// Perform a lookup in the map. The returned boolean indicates if the given key was found.
func (m baseMap) Get(k interface{}) (Element, bool) {
	v, found := m.mp.Get(k)
	if !found {
		return m.Lattice().(latticeWithRange).RngBot(), false
	}

	elem, ok := v.(Element)
	if !ok {
		log.Fatalf("BaseMap did not contain an Element: %T %v\n", v, v)
	}

	return elem, true
}

func (m baseMap) GetOrDefault(k interface{}, def Element) Element {
	if v, found := m.Get(k); found {
		return v
	} else {
		return def
	}
}

func (m baseMap) GetUnsafe(k interface{}) Element {
	if v, found := m.Get(k); found {
		return v
	}

	log.Fatalf("GetUnsafe: %s not found", k)
	panic("Unreachable")
}

func (m baseMap) Find(f func(k interface{}, e Element) bool) (interface{}, Element, bool) {
	iter := m.mp.Iterator()

	for !iter.Done() {
		k, v := iter.Next()
		e, ok := v.(Element)
		if !ok {
			panic("???")
		}

		if f(k, e) {
			return k, e, true
		}
	}

	return nil, nil, false
}

func (m baseMap) Update(loc interface{}, elem Element) baseMap {
	m.mp = m.mp.Set(loc, elem)
	return m
}

func (m baseMap) WeakUpdate(key interface{}, elem Element) baseMap {
	if prev, found := m.Get(key); found {
		return m.Update(key, prev.Join(elem))
	} else {
		return m.Update(key, elem)
	}
}

func (m baseMap) Remove(key interface{}) baseMap {
	m.mp = m.mp.Delete(key)
	return m
}

func (a baseMap) ForEach(do func(interface{}, Element)) {
	for iter := a.mp.Iterator(); !iter.Done(); {
		k, v := iter.Next()
		do(k, v.(Element))
	}
}

func (m baseMap) String() string {
	return m.StringFiltered(func(interface{}) bool { return true })
}

func (m baseMap) StringFiltered(filter func(interface{}) bool) string {
	name := m.Lattice().String()
	length := m.Size()
	if length == 0 {
		return name + ": Empty"
	}

	buf := make([]func() string, 0, length)

	itr := m.mp.Iterator()
	for !itr.Done() {
		k, v := itr.Next()
		if filter(k) {
			buf = append(buf, func() string {
				return fmt.Sprintf("%s ↦ %s", k, v)
			})
		}
	}

	// sort.Slice(buf, func(i, j int) bool {
	// 	return buf[i]() < buf[j]()
	// })
	return i.Indenter().Start(name + ": {").NestThunked(buf...).End("}")
}

// Monomorphic join. Returns BaseMap, skipping a type-conversion.
func (m baseMap) MonoJoin(o baseMap) baseMap {
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
		k, v_itf := itr.Next()
		v := v_itf.(Element)

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

func (m baseMap) Join(o Element) Element {
	checkLatticeMatch(m.lattice, o.Lattice(), "⊔")
	return m.join(o)
}

func (m baseMap) join(o Element) Element {
	switch o := o.(type) {
	case baseMap:
		return m.MonoJoin(o)
	case *DroppedTop:
		return o
	case *LiftedBot:
		return m
	default:
		panic(errInternal)
	}
}

func (m baseMap) Eq(o Element) bool {
	checkLatticeMatch(m.lattice, o.Lattice(), "=")
	return m.eq(o)
}

func (e1 baseMap) eq(e2 Element) bool {
	switch e2 := e2.(type) {
	case baseMap:
		if e1.mp == e2.mp {
			return true
		} else if e1.Size() != e2.Size() {
			return false
		}

		for itr := e1.mp.Iterator(); !itr.Done(); {
			k, v1 := itr.Next()

			v2, found := e2.Get(k.(interface{}))
			if !found || !v1.(Element).eq(v2) {
				return false
			}
		}

		return true
	default:
		return false
	}
}

func (e1 baseMap) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 baseMap) geq(e2 Element) bool {
	return e2.leq(e1) // OBS
}

func (e1 baseMap) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 baseMap) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case baseMap:
		if e1.mp == e2.mp {
			return true
		} else if e1.Size() > e2.Size() {
			return false
		}

		for i := e1.mp.Iterator(); !i.Done(); {
			k, v1 := i.Next()
			v2, found := e2.mp.Get(k)
			if !found || !v1.(Element).leq(v2.(Element)) {
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

func (m baseMap) Meet(o Element) Element {
	panic(errUnsupportedOperation)
}

func (m baseMap) meet(o Element) Element {
	panic(errUnsupportedOperation)
}
