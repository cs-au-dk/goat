//go:build ignore
// +build ignore

package lattice

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
	base baseMap
}

// Map methods
func (w WrappedMapElement) Size() int {
	return w.base.Size()
}

func (w WrappedMapElement) Height() int {
	return w.base.Height()
}

func (w WrappedMapElement) Get(key KEYTYPE) (VALUETYPE, bool) {
	v, found := w.base.Get(key)
	return v.(VALUETYPE), found
}

func (w WrappedMapElement) GetOrDefault(key KEYTYPE, dflt VALUETYPE) VALUETYPE {
	return w.base.GetOrDefault(key, dflt).(VALUETYPE)
}

func (w WrappedMapElement) GetUnsafe(key KEYTYPE) VALUETYPE {
	return w.base.GetUnsafe(key).(VALUETYPE)
}

func (w WrappedMapElement) Update(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.base = w.base.Update(key, value)
	return w
}

func (w WrappedMapElement) WeakUpdate(key KEYTYPE, value VALUETYPE) WrappedMapElement {
	w.base = w.base.WeakUpdate(key, value)
	return w
}

func (w WrappedMapElement) Remove(key KEYTYPE) WrappedMapElement {
	w.base = w.base.Remove(key)
	return w
}

func (w WrappedMapElement) ForEach(f func(KEYTYPE, VALUETYPE)) {
	w.base.ForEach(func(key interface{}, value Element) {
		f(key.(KEYTYPE), value.(VALUETYPE))
	})
}

func (w WrappedMapElement) Find(f func(KEYTYPE, VALUETYPE) bool) (zk KEYTYPE, zv VALUETYPE, b bool) {
	k, e, found := w.base.Find(func(k interface{}, e Element) bool {
		return f(k.(KEYTYPE), e.(VALUETYPE))
	})
	if found {
		return k.(KEYTYPE), e.(VALUETYPE), true
	}
	return zk, zv, b
}

// Lattice element methods
func (w WrappedMapElement) Leq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊑")
	return w.leq(e)
}

func (w WrappedMapElement) leq(e Element) bool {
	return w.base.leq(e.(WrappedMapElement).base)
}

func (w WrappedMapElement) Geq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊒")
	return w.geq(e)
}

func (w WrappedMapElement) geq(e Element) bool {
	return w.base.geq(e.(WrappedMapElement).base)
}

func (w WrappedMapElement) Eq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "=")
	return w.eq(e)
}

func (w WrappedMapElement) eq(e Element) bool {
	return w.base.eq(e.(WrappedMapElement).base)
}

func (w WrappedMapElement) Join(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊔")
	return w.join(o)
}

func (w WrappedMapElement) join(o Element) Element {
	return w.MonoJoin(o.(WrappedMapElement))
}

func (w WrappedMapElement) MonoJoin(o WrappedMapElement) WrappedMapElement {
	w.base = w.base.MonoJoin(o.base)
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
