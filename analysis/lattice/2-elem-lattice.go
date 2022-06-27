package lattice

type TwoElementLattice struct {
	lattice
}

func (latticeFactory) TwoElement() *TwoElementLattice {
	return twoElementLattice.TwoElement()
}

var twoElementLattice *TwoElementLattice = &TwoElementLattice{}

func (*TwoElementLattice) Top() Element {
	return twoElemTop
}

func (*TwoElementLattice) Bot() Element {
	return twoElemBot
}

func (*TwoElementLattice) TwoElement() *TwoElementLattice {
	// Will always succeed.
	return twoElementLattice
}

func (l1 *TwoElementLattice) Eq(l2 Lattice) bool {
	// First try to get away with referential equality
	if l1 == l2 {
		return true
	}
	switch l2 := l2.(type) {
	case *TwoElementLattice:
		return true
	case *Dropped:
		return l1.Eq(l2.Lattice)
	case *Lifted:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (*TwoElementLattice) String() string {
	return colorize.Lattice("𝔹")
}

func (*TwoElementLattice) Elements() []Element {
	return []Element{twoElemBot, twoElemTop}
}
