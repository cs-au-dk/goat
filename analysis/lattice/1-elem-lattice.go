package lattice

type OneElementLattice struct {
	lattice
}

func (latticeFactory) OneElement() *OneElementLattice {
	return oneElementLattice
}

var oneElementLattice *OneElementLattice = &OneElementLattice{}

func (*OneElementLattice) Top() Element {
	return oneElem
}

func (*OneElementLattice) Bot() Element {
	return oneElem
}

func (*OneElementLattice) OneElement() *OneElementLattice {
	// Will always succeed.
	return oneElementLattice
}

func (l1 *OneElementLattice) Eq(l2 Lattice) bool {
	// First try to get away with referential equality
	if l1 == l2 {
		return true
	}
	switch l2 := l2.(type) {
	case *OneElementLattice:
		return true
	case *Dropped:
		return l1.Eq(l2.Lattice)
	case *Lifted:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (*OneElementLattice) String() string {
	return colorize.Lattice("𝕀")
}

func (*OneElementLattice) Elements() []Element {
	return []Element{oneElem}
}
