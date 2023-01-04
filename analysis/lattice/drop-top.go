package lattice

import "strconv"

// DroppedTop represents a synthetic ⊤ element obtained by dropping a lattice.
type DroppedTop struct {
	element
}

func (e *DroppedTop) String() string {
	return colorize.Element("T") + strconv.Itoa(e.Index())
}

// The Height of a dropped element is -1, as it is contingent
func (e *DroppedTop) Height() int {
	return -1
}

// Index reveals how often a lattice has been dropped.
func (e *DroppedTop) Index() int {
	lat := e.lattice.Dropped()
	return lat.index
}

// Eq computes ⊤ = x, where x is a lattice element.
// Performs lattice type checking.
func (e1 *DroppedTop) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes ⊤ = x, where x is a lattice element.
func (e1 *DroppedTop) eq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *DroppedTop:
		return e1.Index() == e2.Index()
	}
	return false
}

// Geq computes ⊤ ⊒ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 *DroppedTop) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes ⊤ ⊒ x, where x is a lattice element.
func (e1 *DroppedTop) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *DroppedTop:
		return e1.Index() >= e2.Index()
	}
	return true
}

// Leq computes ⊤ ⊑ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 *DroppedTop) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes ⊤ ⊑ x, where x is a lattice element.
func (e1 *DroppedTop) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *DroppedTop:
		return e1.Index() <= e2.Index()
	}
	return false
}

// Join computes ⊤ ⊔ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 *DroppedTop) Join(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes ⊤ ⊔ x, where x is a lattice element.
func (e1 *DroppedTop) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case *DroppedTop:
		if e1.Index() < e2.Index() {
			return e2
		}
	}

	return e1
}

// Meet computes ⊤ ⊓ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 *DroppedTop) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes ⊤ ⊓ x, where x is a lattice element.
func (e1 *DroppedTop) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case *DroppedTop:
		if e1.Index() < e2.Index() {
			return e1
		}
	}

	return e2
}
