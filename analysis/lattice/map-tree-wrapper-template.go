//go:build ignore
// +build ignore

package lattice

import (
	"github.com/cs-au-dk/goat/utils/tree"
	"log"
)

type WrappedMapElementLattice struct {
	mapLatticeBase
}

func (m *WrappedMapElementLattice) Eq(o Lattice) bool {
	switch o := o.(type) {
	case *WrappedMapElementLattice:
		return true
	case *Lifted:
		return m.Eq(o.Lattice)
	case *Dropped:
		return m.Eq(o.Lattice)
	default:
		return false
	}
}

func (m *WrappedMapElementLattice) Top() Element {
	panic(errUnsupportedOperation)
}

func (m *WrappedMapElementLattice) WrappedMapElement() *WrappedMapElementLattice {
	return m
}

type WrappedMapElement struct {
	element
	base tree.Tree[KEYTYPE, VALUETYPE]
}

// Map methods
func (w WrappedMapElement) Size() int {
	return w.base.Size()
}

/*
func (w WrappedMapElement) Height() int {
	return w.base.Height()
}
*/

func (w WrappedMapElement) Get(key KEYTYPE) (VALUETYPE, bool) {
	v, found := w.base.Lookup(key)
	if !found {
		return w.Lattice().(latticeWithRange).RngBot().(VALUETYPE), false
	}

	return v, true
}

func (w WrappedMapElement) GetOrDefault(key KEYTYPE, dflt VALUETYPE) VALUETYPE {
	if v, found := w.Get(key); found {
		return v
	} else {
		return dflt
	}
}

func (w WrappedMapElement) GetUnsafe(key KEYTYPE) VALUETYPE {
	if v, found := w.Get(key); found {
		return v
	}

	log.Fatalf("GetUnsafe: %v not found", key)
	panic("Unreachable")
}

func (w WrappedMapElement) Update(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.base = w.base.InsertOrMerge(key, value, func(elem, old VALUETYPE) (VALUETYPE, bool) {
		if elem.eq(old) {
			return old, true
		} else {
			return elem, false
		}
	})
	return w
}

var _WrappedMapElementMergeFunc = func(elem, old VALUETYPE) (VALUETYPE, bool) {
	if elem.eq(old) {
		return old, true
	} else {
		return elem.MonoJoin(old), false
	}
}

func (w WrappedMapElement) WeakUpdate(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.base = w.base.InsertOrMerge(key, value, _WrappedMapElementMergeFunc)
	return w
}

func (w WrappedMapElement) Remove(key KEYTYPE) WrappedMapElement {
	w.base = w.base.Remove(key)
	return w
}

func (w WrappedMapElement) ForEach(f func(KEYTYPE, VALUETYPE)) {
	w.base.ForEach(f)
}

/*
func (w WrappedMapElement) Find(f func(KEYTYPE, VALUETYPE) bool) (zk KEYTYPE, zv VALUETYPE, b bool) {
	k, e, found := w.base.Find(func(k KEYTYPE, e Element) bool {
		return f(k, e.(VALUETYPE))
	})
	if found {
		return k, e.(VALUETYPE), true
	}
	return zk, zv, b
}
*/

// Lattice element methods
func (w WrappedMapElement) Leq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊑")
	return w.leq(e)
}

func (w WrappedMapElement) leq(e Element) bool {
	return w.join(e).eq(e)
}

func (w WrappedMapElement) Geq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊒")
	return w.geq(e)
}

func (w WrappedMapElement) geq(e Element) bool {
	return e.leq(w) // OBS
}

func (w WrappedMapElement) Eq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "=")
	return w.eq(e)
}

func (w WrappedMapElement) eq(e Element) bool {
	return w.base.Equal(e.(WrappedMapElement).base, func(a, b VALUETYPE) bool {
		return a.eq(b)
	})
}

func (w WrappedMapElement) Join(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊔")
	return w.join(o)
}

func (w WrappedMapElement) join(o Element) Element {
	return w.MonoJoin(o.(WrappedMapElement))
}

func (w WrappedMapElement) MonoJoin(o WrappedMapElement) WrappedMapElement {
	w.base = w.base.Merge(o.base, _WrappedMapElementMergeFunc)
	return w
}

func (w WrappedMapElement) Meet(o Element) Element {
	panic(errUnsupportedOperation)
}

func (w WrappedMapElement) meet(o Element) Element {
	panic(errUnsupportedOperation)
}

func (w WrappedMapElement) String() string {
	return w.base.String()
}

// Type conversion
func (w WrappedMapElement) WrappedMapElement() WrappedMapElement {
	return w
}
