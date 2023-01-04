//go:build ignore
// +build ignore

package lattice

// WrappedMapElementLattice WrappedDescriptionMapLattice
// This map has a finite number of keys.
type WrappedMapElementLattice struct {
	lattice
	mp MapLattice[KEYTYPE]
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

// Top will return the InformalMapLattice equivalent to the ⊤ element.
func (m *WrappedMapElementLattice) Top() Element {
	return WrappedMapElement{
		element{m},
		m.mp.Top().(Map[KEYTYPE]),
	}
}

// Bot will return the InformalMapLattice equivalent to the ⊥ element.
func (m *WrappedMapElementLattice) Bot() Element {
	return WrappedMapElement{
		element{m},
		m.mp.Bot().(Map[KEYTYPE]),
	}
}

// WrappedMapElement safely converts to the InformalMapLattice.
func (m *WrappedMapElementLattice) WrappedMapElement() *WrappedMapElementLattice {
	return m
}

// WrappedMapElement WrappedDescriptionMapElement
type WrappedMapElement struct {
	element
	mp Map[KEYTYPE]
}

// Get retrieves the InformalValueValue at the given InformalKeyValue.
// The attached boolean is false if the InformalKeyValue is not found.
func (w WrappedMapElement) Get(key KEYTYPE) VALUETYPE {
	return w.mp.Get(key).(VALUETYPE)
}

// Update changes the binding for the given InformalKeyValue with the InformalValueValue.
func (w WrappedMapElement) Update(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.mp = w.mp.update(key, value)
	return w
}

func (w WrappedMapElement) ForEach(f func(KEYTYPE, VALUETYPE)) {
	w.mp.ForEach(func(key KEYTYPE, value Element) {
		f(key, value.(VALUETYPE))
	})
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Leq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊑")
	return w.leq(e)
}

// leq computes m ⊑ o.
func (w WrappedMapElement) leq(e Element) bool {
	return w.mp.leq(e.(WrappedMapElement).mp)
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Geq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊒")
	return w.geq(e)
}

// geq computes m ⊒ o.
func (w WrappedMapElement) geq(e Element) bool {
	return w.mp.geq(e.(WrappedMapElement).mp)
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Eq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "=")
	return w.eq(e)
}

// eq computes m = o.
func (w WrappedMapElement) eq(e Element) bool {
	return w.mp.eq(e.(WrappedMapElement).mp)
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Join(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊔")
	return w.join(o)
}

// join computes m ⊔ o.
func (w WrappedMapElement) join(o Element) Element {
	switch o := o.(type) {
	case WrappedMapElement:
		return w.MonoJoin(o)
	case *LiftedBot:
		return w
	case *DroppedTop:
		return o
	default:
		panic(errInternal)
	}
}

// MonoJoin is the monomorphic variant of m ⊔ o for InformalMapElement.
func (w WrappedMapElement) MonoJoin(o WrappedMapElement) WrappedMapElement {
	w.mp = w.mp.MonoJoin(o.mp)
	return w
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (w WrappedMapElement) Meet(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊓")
	return w.meet(o)
}

// meet computes m ⊓ o.
func (w WrappedMapElement) meet(o Element) Element {
	switch o := o.(type) {
	case WrappedMapElement:
		return w.MonoMeet(o)
	case *LiftedBot:
		return o
	case *DroppedTop:
		return w
	default:
		panic(errInternal)
	}
}

// MonoMeet is the monomorphic variant of m ⊓ o for members of the InformalMapLattice.
func (w WrappedMapElement) MonoMeet(o WrappedMapElement) WrappedMapElement {
	w.mp = w.mp.MonoMeet(o.mp)
	return w
}

func (w WrappedMapElement) String() string {
	return w.mp.String()
}

// WrappedMapElement safely converts to a member of the InformalMapLattice.
func (w WrappedMapElement) WrappedMapElement() WrappedMapElement {
	return w
}
