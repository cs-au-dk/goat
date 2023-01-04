package lattice

// TwoElementLattice represents the two element lattice:
//
//	⊤
//	|
//	⊥
type TwoElementLattice struct {
	lattice
}

// TwoElement returns the two element lattice.
func (latticeFactory) TwoElement() *TwoElementLattice {
	return twoElementLattice
}

// twoElementLattice is a singleton instantiation of the two-element lattice.
var twoElementLattice *TwoElementLattice = &TwoElementLattice{}

// Top retrieves the ⊤ element of the two-element lattice.
func (*TwoElementLattice) Top() Element {
	return twoElemTop
}

// Top retrieves the ⊥ element of the two-element lattice.
func (*TwoElementLattice) Bot() Element {
	return twoElemBot
}

// TwoElement converts the two-element lattice to its concrete type form.
// Is used when the two-element lattice is masked by the Lattice interface.
func (*TwoElementLattice) TwoElement() *TwoElementLattice {
	// Will always succeed.
	return twoElementLattice
}

// Eq checks that l2 is the two-element lattice.
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
	return colorize.Lattice("⌶")
}
