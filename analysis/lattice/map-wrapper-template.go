//go:build ignore
// +build ignore

package lattice

// WrappedMapElementLattice WrappedDescriptionMapLattice
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

// Top will panic for the InformalMapLattice, as its ⊤ element is unbounded in size.
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
	base baseMap[KEYTYPE]
}

// Size returns the number of keys bound to non-⊥ elements in the InformalMapElement.
func (w WrappedMapElement) Size() int {
	return w.base.Size()
}

// Height computes the height of the InformalMapElement in its corresponding lattice.
func (w WrappedMapElement) Height() int {
	return w.base.Height()
}

// Get retrieves the InformalValueValue at the given InformalKeyValue.
// The attached boolean is false if the InformalKeyValue is not found.
func (w WrappedMapElement) Get(key KEYTYPE) (VALUETYPE, bool) {
	v, found := w.base.Get(key)
	return v.(VALUETYPE), found
}

// GetOrDefault retrieves the InformalValueValue at the given InformalKeyValue,
// or the default value, if the key is unbound.
func (w WrappedMapElement) GetOrDefault(key KEYTYPE, dflt VALUETYPE) VALUETYPE {
	return w.base.GetOrDefault(key, dflt).(VALUETYPE)
}

// GetUnsafe retrieves the InformalValueValue at the given InformalKeyValue.
// Throws a fatal exception if the key is unbound.
func (w WrappedMapElement) GetUnsafe(key KEYTYPE) VALUETYPE {
	return w.base.GetUnsafe(key).(VALUETYPE)
}

// Update changes the binding for the given InformalKeyValue with the InformalValueValue.
func (w WrappedMapElement) Update(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.base = w.base.Update(key, value)
	return w
}

// WeakUpdate computes the least-upper bound between the given InformalValueValue and
// the InformalValueValue at the given InformalKeyValue.
func (w WrappedMapElement) WeakUpdate(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.base = w.base.WeakUpdate(key, value)
	return w
}

// Remove unbinds the InformalValueValue at the given InformalKeyValue.
func (w WrappedMapElement) Remove(key KEYTYPE) WrappedMapElement {
	w.base = w.base.Remove(key)
	return w
}

// ForEach executes the given procedure for each key-value pair in the InformalMapElement.
func (w WrappedMapElement) ForEach(f func(KEYTYPE, VALUETYPE)) {
	w.base.ForEach(func(key KEYTYPE, value Element) {
		f(key, value.(VALUETYPE))
	})
}

// Find retrieves an arbitrary key-value pair in the InformalMapElement.
func (w WrappedMapElement) Find(f func(KEYTYPE, VALUETYPE) bool) (zk KEYTYPE, zv VALUETYPE, b bool) {
	k, e, found := w.base.Find(func(k KEYTYPE, e Element) bool {
		return f(k, e.(VALUETYPE))
	})
	if found {
		return k, e.(VALUETYPE), true
	}
	return zk, zv, b
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Leq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊑")
	return w.leq(e)
}

// leq computes m ⊑ o.
func (w WrappedMapElement) leq(e Element) bool {
	return w.base.leq(e.(WrappedMapElement).base)
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Geq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊒")
	return w.geq(e)
}

// geq computes m ⊒ o.
func (w WrappedMapElement) geq(e Element) bool {
	return w.base.geq(e.(WrappedMapElement).base)
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Eq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "=")
	return w.eq(e)
}

// eq computes m = o.
func (w WrappedMapElement) eq(e Element) bool {
	return w.base.eq(e.(WrappedMapElement).base)
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

// MonoJoin is a monomorphic variant of m ⊔ o for InformalMapElement.
func (w WrappedMapElement) MonoJoin(o WrappedMapElement) WrappedMapElement {
	w.base = w.base.MonoJoin(o.base)
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
