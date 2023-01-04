//go:build ignore
// +build ignore

package lattice

import (
	"github.com/cs-au-dk/goat/utils/tree"
	"log"
)

// WrappedMapElementLattice WrappedDescriptionMapLattice
// The implementation is based on Patricia trees.
type WrappedMapElementLattice struct {
	mapLatticeBase
}

// Eq checks for lattice equality for the InformalMapLattice.
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

// Top will panic for InformalMapLattice, as its ⊤ element is unbounded in size.
func (m *WrappedMapElementLattice) Top() Element {
	panic(errUnsupportedOperation)
}

// WrappedMapElement safely converts to the InformalMapLattice.
func (m *WrappedMapElementLattice) WrappedMapElement() *WrappedMapElementLattice {
	return m
}

// WrappedMapElement WrappedDescriptionMapElement
type WrappedMapElement struct {
	element
	base tree.Tree[KEYTYPE, VALUETYPE]
}

// Size returns the number of keys bound to non-⊥ elements in the InformalMapElement.
func (w WrappedMapElement) Size() int {
	return w.base.Size()
}

// Get retrieves the InformalValueValue at the given InformalKeyValue.
// The attached boolean is false if the InformalKeyValue is not found.
func (w WrappedMapElement) Get(key KEYTYPE) (VALUETYPE, bool) {
	v, found := w.base.Lookup(key)
	if !found {
		return w.Lattice().(latticeWithRange).RngBot().(VALUETYPE), false
	}

	return v, true
}

// GetOrDefault retrieves the InformalValueValue at the given InformalKeyValue,
// or the default value, if the key is unbound.
func (w WrappedMapElement) GetOrDefault(key KEYTYPE, dflt VALUETYPE) VALUETYPE {
	if v, found := w.Get(key); found {
		return v
	} else {
		return dflt
	}
}

// GetUnsafe retrieves the InformalValueValue at the given InformalKeyValue.
// Throws a fatal exception if the key is unbound.
func (w WrappedMapElement) GetUnsafe(key KEYTYPE) VALUETYPE {
	if v, found := w.Get(key); found {
		return v
	}

	log.Fatalf("GetUnsafe: %v not found", key)
	panic("Unreachable")
}

// Update changes the binding for the given InformalKeyValue with the InformalValueValue.
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

// _WrappedMapElementMergeFunc is a customized implementation of the merge function
// that checks for equality with the old value before merging for added efficiency.
var _WrappedMapElementMergeFunc = func(elem, old VALUETYPE) (VALUETYPE, bool) {
	if elem.eq(old) {
		return old, true
	} else {
		return elem.MonoJoin(old), false
	}
}

// WeakUpdate computes the least-upper bound between the given InformalValueValue and
// the InformalValueValue at the given InformalKeyValue.
func (w WrappedMapElement) WeakUpdate(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.base = w.base.InsertOrMerge(key, value, _WrappedMapElementMergeFunc)
	return w
}

// Remove unbinds the InformalValueValue at the given InformalKeyValue.
func (w WrappedMapElement) Remove(key KEYTYPE) WrappedMapElement {
	w.base = w.base.Remove(key)
	return w
}

// ForEach executes the given procedure for each key-value pair in the InformalMapElement.
func (w WrappedMapElement) ForEach(f func(KEYTYPE, VALUETYPE)) {
	w.base.ForEach(f)
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Leq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊑")
	return w.leq(e)
}

// leq computes m ⊑ o.
func (w WrappedMapElement) leq(e Element) bool {
	return w.join(e).eq(e)
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Geq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊒")
	return w.geq(e)
}

// geq computes m ⊒ o.
func (w WrappedMapElement) geq(e Element) bool {
	return e.leq(w) // OBS
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Eq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "=")
	return w.eq(e)
}

// eq computes m = o.
func (w WrappedMapElement) eq(e Element) bool {
	return w.base.Equal(e.(WrappedMapElement).base, func(a, b VALUETYPE) bool {
		return a.eq(b)
	})
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Join(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊔")
	return w.join(o)
}

// join computes m ⊔ o.
func (w WrappedMapElement) join(o Element) Element {
	return w.MonoJoin(o.(WrappedMapElement))
}

// MonoJoin is the monomorphic variant of m ⊔ o for InformalMapElement.
func (w WrappedMapElement) MonoJoin(o WrappedMapElement) WrappedMapElement {
	w.base = w.base.Merge(o.base, _WrappedMapElementMergeFunc)
	return w
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Meet(o Element) Element {
	panic(errUnsupportedOperation)
}

// meet computes m ⊓ o.
func (w WrappedMapElement) meet(o Element) Element {
	panic(errUnsupportedOperation)
}

func (w WrappedMapElement) String() string {
	return w.base.String()
}

// WrappedMapElement safely converts to a member of the InformalMapLattice.
func (w WrappedMapElement) WrappedMapElement() WrappedMapElement {
	return w
}
