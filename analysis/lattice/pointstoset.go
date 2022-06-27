package lattice

import (
	"fmt"
	"sort"

	loc "Goat/analysis/location"
	i "Goat/utils/indenter"
	"Goat/utils/tree"
)

// A points to set element contains a set of locations internally represented
// as a map from location to struct{}.
// We enforce points-to sets to be in canonical form. That is: no location
// in the points-to set is represented by another in the same set.
type PointsTo struct {
	element
	mem tree.Tree[loc.Location, struct{}]
}

// Returns a canonicalized points-to set with the given locations.
// Faster than running Add for each location (which can take quadratic time).
func (elementFactory) PointsTo(locs ...loc.Location) PointsTo {
	p := pointsToLattice.Bot().PointsTo()
	var doubleCheck []loc.Location
	for _, l := range locs {
		if _, hasRep := recRepresentative(l); hasRep {
			doubleCheck = append(doubleCheck, l)
		} else {
			p.mem = p.mem.Insert(l, struct{}{})
		}
	}
	for _, l := range doubleCheck {
		// Adding non-top locations takes constant time.
		p = p.Add(l)
	}
	return p
}

func (m PointsTo) Size() int {
	return m.mem.Size()
}

func (m PointsTo) Height() int {
	return m.mem.Size()
}

func (m PointsTo) Empty() bool {
	return m.Size() == 0
}

func (m PointsTo) Entries() (ret []loc.Location) {
	m.mem.ForEach(func(k loc.Location, _ struct{}) {
		ret = append(ret, k)
	})

	return ret
}

func (m PointsTo) ForEach(do func(loc.Location)) {
	m.mem.ForEach(func(k loc.Location, _ struct{}) {
		do(k.(loc.Location))
	})
}

func (m PointsTo) NonNilEntries() []loc.Location {
	return m.Remove(loc.NilLocation{}).Entries()
}

func (p PointsTo) Contains(key loc.Location) bool {
	_, found := p.mem.Lookup(key)
	return found
}

// Return a points-to set where the nil location has been removed.
// If the nil location is found in the points-to set, execute the provided
// procedure
func (p PointsTo) FilterNilCB(onNilFound func()) PointsTo {
	nl := loc.NilLocation{}

	if p.Contains(nl) {
		onNilFound()
		return p.Remove(nl)
	}
	return p
}

// Return a points-to set where the nil location has been removed.
func (p PointsTo) FilterNil() PointsTo {
	return p.FilterNilCB(func() {})
}

// Filter all the loctions in a points-to set. Keep only those
// that satisfy the predicate
func (p PointsTo) Filter(pred func(l loc.Location) bool) PointsTo {
	p.mem.ForEach(func(l loc.Location, _ struct{}) {
		if !pred(l) {
			p = p.Remove(l)
		}
	})

	return p
}

func (p PointsTo) HasNil() bool {
	return p.Contains(loc.NilLocation{})
}

func (p PointsTo) Add(ptr loc.Location) PointsTo {
	if p.Contains(ptr) {
		return p
	}

	if IsTopLocation(ptr) {
		// Remove all locations that are represented by ptr
		p = p.Filter(func(ol loc.Location) bool {
			if rep, hasRep := recRepresentative(ol); hasRep && ptr == rep {
				return false
			}
			return true
		})
	} else if rep, hasRep := recRepresentative(ptr); hasRep {
		// If p already contains the top representative, do not add the location.
		if p.Contains(rep) {
			return p
		}
	}

	p.mem = p.mem.Insert(ptr, struct{}{})

	return p
}

/* NOTE: We don't need this currently.
func (p PointsTo) Union(locs ...loc.Location) PointsTo {
	return p.MonoJoin(elFact.PointsTo(locs...))
}
*/

func (p PointsTo) Remove(loc loc.Location) PointsTo {
	p.mem = p.mem.Remove(loc)
	return p
}

func (m PointsTo) String() string {
	buf := []fmt.Stringer{}

	m.ForEach(func(k loc.Location) {
		buf = append(buf, k)
	})

	if len(buf) == 0 {
		return colorize.Element("∅")
	}

	sort.Slice(buf, func(i, j int) bool {
		return buf[i].String() < buf[j].String()
	})
	return i.Indenter().Start("{").
		NestSep(",", buf...).
		End("}")
}

func (m PointsTo) Join(o Element) Element {
	checkLatticeMatch(m.Lattice(), o.Lattice(), "⊔")
	return m.join(o)
}

func (m PointsTo) join(o Element) Element {
	switch o := o.(type) {
	case PointsTo:
		return m.MonoJoin(o)
	case *LiftedBot:
		return m
	case *DroppedTop:
		return o
	default:
		panic(errInternal)
	}
}

func (m PointsTo) MonoJoin(o PointsTo) PointsTo {
	m.mem = m.mem.Merge(o.mem, func(_, b struct{}) (struct{}, bool) {
		return b, true
	})
	return m.Filter(func(l loc.Location) bool {
		rep, hasRep := recRepresentative(l)
		return !hasRep || !m.Contains(rep)
	})
}

func (m PointsTo) Meet(o Element) Element {
	checkLatticeMatch(m.Lattice(), o.Lattice(), "⊓")
	return m.meet(o)
}

func (m PointsTo) meet(o Element) Element {
	switch o := o.(type) {
	case PointsTo:
		return m.MonoMeet(o)
	case *LiftedBot:
		return o
	case *DroppedTop:
		return m
	default:
		panic(errInternal)
	}
}

func (m PointsTo) MonoMeet(o PointsTo) PointsTo {
	ret := m.Filter(func(l loc.Location) bool {
		if rep, hasRep := recRepresentative(l); hasRep && o.Contains(rep) {
			// We have a location that is represented by a top location in the
			// other set. The meet of those is the non-top location.
			return true
		}
		_, found := o.mem.Lookup(l)
		return found
	})

	// Check if the other set has some locations that are represented by top
	// locations in our set.
	o.mem.ForEach(func(l loc.Location, v struct{}) {
		if rep, hasRep := recRepresentative(l); hasRep && m.Contains(rep) {
			ret.mem = ret.mem.Insert(l, v)
		}
	})

	return ret
}

func (m PointsTo) Eq(o Element) bool {
	checkLatticeMatch(m.Lattice(), o.Lattice(), "=")
	return m.eq(o)
}

func (m PointsTo) eq(oe Element) bool {
	switch o := oe.(type) {
	case PointsTo:
		return m.mem.Equal(o.mem, func(_, _ struct{}) bool { return true })
	case *LiftedBot:
		return false
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

// TODO: These methods could be optimized

func (m PointsTo) Geq(o Element) bool {
	checkLatticeMatch(m.Lattice(), o.Lattice(), "⊒")
	return m.geq(o)
}

func (m PointsTo) geq(o Element) bool {
	switch o := o.(type) {
	case PointsTo:
		return o.leq(m) // OBS
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

func (m PointsTo) Leq(o Element) bool {
	checkLatticeMatch(m.Lattice(), o.Lattice(), "⊑")
	return m.leq(o)
}

func (m PointsTo) leq(o Element) bool {
	switch o := o.(type) {
	case PointsTo:
		return m.MonoMeet(o).eq(m)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

/* Element boilerplate */
func (m PointsTo) PointsTo() PointsTo {
	return m
}
