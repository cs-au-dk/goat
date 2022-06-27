package lattice

import "strconv"

type LiftedBot struct {
	element
}

func (e *LiftedBot) Lifted() *LiftedBot {
	return e
}

func (e *LiftedBot) String() string {
	return colorize.Element("\u22A5") + strconv.Itoa(e.Index())
}

func (e *LiftedBot) Height() int {
	return 0
}

func (e *LiftedBot) Index() int {
	lat := e.lattice.(*Lifted)
	return lat.index
}

func (e1 *LiftedBot) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 *LiftedBot) eq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *LiftedBot:
		return e1.Index() == e2.Index()
	}
	return false
}

func (e1 *LiftedBot) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 *LiftedBot) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *LiftedBot:
		return e1.Index() <= e2.Index()
	}
	return false
}

func (e1 *LiftedBot) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 *LiftedBot) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *LiftedBot:
		return e1.Index() >= e2.Index()
	}
	return true
}

func (e1 *LiftedBot) Join(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 *LiftedBot) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case *LiftedBot:
		if e1.Index() < e2.Index() {
			return e1
		}
	}

	return e2
}

func (e1 *LiftedBot) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 *LiftedBot) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case *LiftedBot:
		if e1.Index() < e2.Index() {
			return e2
		}
	}

	return e1
}
