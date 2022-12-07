// +build ignore

package lattice

type WrappedMapElementLattice struct {
	lattice
	mp MapLattice[KEYTYPE]
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
	return WrappedMapElement{
		element{m},
		m.mp.Top().(Map[KEYTYPE]),
	}
}

func (m *WrappedMapElementLattice) Bot() Element {
	return WrappedMapElement{
		element{m},
		m.mp.Bot().(Map[KEYTYPE]),
	}
}

func (m *WrappedMapElementLattice) WrappedMapElement() *WrappedMapElementLattice {
	return m
}

type WrappedMapElement struct {
	element
	mp Map[KEYTYPE]
}

// Map methods
func (w WrappedMapElement) Get(key KEYTYPE) VALUETYPE {
	return w.mp.Get(key).(VALUETYPE)
}

func (w WrappedMapElement) Update(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.mp = w.mp.update(key, value)
	return w
}

func (w WrappedMapElement) ForEach(f func(KEYTYPE, VALUETYPE)) {
	w.mp.ForEach(func(key KEYTYPE, value Element) {
		f(key, value.(VALUETYPE))
	})
}

// Lattice element methods
func (w WrappedMapElement) Leq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊑")
	return w.leq(e)
}

func (w WrappedMapElement) leq(e Element) bool {
	return w.mp.leq(e.(WrappedMapElement).mp)
}

func (w WrappedMapElement) Geq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊒")
	return w.geq(e)
}

func (w WrappedMapElement) geq(e Element) bool {
	return w.mp.geq(e.(WrappedMapElement).mp)
}

func (w WrappedMapElement) Eq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "=")
	return w.eq(e)
}

func (w WrappedMapElement) eq(e Element) bool {
	return w.mp.eq(e.(WrappedMapElement).mp)
}

func (w WrappedMapElement) Join(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊔")
	return w.join(o)
}

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

func (w WrappedMapElement) MonoJoin(o WrappedMapElement) WrappedMapElement {
	w.mp = w.mp.MonoJoin(o.mp)
	return w
}

func (w WrappedMapElement) Meet(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊓")
	return w.meet(o)
}

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

func (w WrappedMapElement) MonoMeet(o WrappedMapElement) WrappedMapElement {
	w.mp = w.mp.MonoMeet(o.mp)
	return w
}

func (w WrappedMapElement) String() string {
	return w.mp.String()
}

// Type conversion
func (w WrappedMapElement) WrappedMapElement() WrappedMapElement {
	return w
}
