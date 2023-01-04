package lattice

// IntervalLattice represents the interval lattice.
type IntervalLattice struct {
	lattice
}

// intervalLattice is a singleton instantiation of the interval lattice.
var intervalLattice = &IntervalLattice{}

// Interval yields the interval lattice.
func (latticeFactory) Interval() *IntervalLattice {
	return intervalLattice
}

// Top yields [-∞, +∞].
func (*IntervalLattice) Top() Element {
	return Interval{
		low:  MinusInfinity{},
		high: PlusInfinity{},
	}
}

// Bot yields [+∞, -∞].
func (*IntervalLattice) Bot() Element {
	return Interval{
		low:  PlusInfinity{},
		high: MinusInfinity{},
	}
}

func (*IntervalLattice) String() string {
	return "[" + colorize.Lattice("ℤ") +
		", " + colorize.Lattice("ℤ") + "]"
}

// Eq checks for equality with another lattice.
func (l1 *IntervalLattice) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	case *IntervalLattice:
		return true
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

// Interval safely converts the interval lattice to IntervalLattice.
func (l1 *IntervalLattice) Interval() *IntervalLattice {
	return l1
}
