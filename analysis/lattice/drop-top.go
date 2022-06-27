package lattice

import "strconv"

type DroppedTop struct {
	element
}

func (e *DroppedTop) String() string {
	return colorize.Element("T") + strconv.Itoa(e.Index())
}

func (e *DroppedTop) Height() int {
	return -1
}

func (e *DroppedTop) Index() int {
	lat := e.lattice.Dropped()
	return lat.index
}

func (e1 *DroppedTop) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 *DroppedTop) eq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *DroppedTop:
		return e1.Index() == e2.Index()
	}
	return false
}

func (e1 *DroppedTop) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 *DroppedTop) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *DroppedTop:
		return e1.Index() >= e2.Index()
	}
	return true
}

func (e1 *DroppedTop) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 *DroppedTop) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *DroppedTop:
		return e1.Index() <= e2.Index()
	}
	return false
}

func (e1 *DroppedTop) Join(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 *DroppedTop) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case *DroppedTop:
		if e1.Index() < e2.Index() {
			return e2
		}
	}

	return e1
}

func (e1 *DroppedTop) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 *DroppedTop) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case *DroppedTop:
		if e1.Index() < e2.Index() {
			return e1
		}
	}

	return e2
}
