package lattice

type IntervalLattice struct {
	lattice
}

var intervalLattice = &IntervalLattice{}

func (latticeFactory) Interval() *IntervalLattice {
	return intervalLattice
}

func (*IntervalLattice) Top() Element {
	return Interval{
		low:  MinusInfinity{},
		high: PlusInfinity{},
	}
}

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

func (l1 *IntervalLattice) Interval() *IntervalLattice {
	return l1
}
