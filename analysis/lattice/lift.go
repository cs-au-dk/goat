package lattice

func Lift(lat Lattice) *Lifted {
	var getIndex func(Lattice) int
	// For a multi-dropped/lifted lattice, dig for the index of the
	// surface-most lift.
	getIndex = func(lat Lattice) int {
		switch lat := lat.(type) {
		case *Lifted:
			return lat.index + 1
		case *Dropped:
			return getIndex(lat.Lattice)
		}
		return 0
	}

	lifted := new(Lifted)

	lifted.Lattice = lat
	lifted.index = getIndex(lat)
	return lifted
}

type Lifted struct {
	Lattice
	// An index above 0 signifies a multi-lifted lattice
	index int
	bot   *LiftedBot
}

func (l *Lifted) Dropped() *Dropped {
	return l.Lattice.Dropped()
}

func (l *Lifted) Preheight() int {
	return 1 + l.Lattice.Preheight()
}

func (l *Lifted) Lifted() *Lifted {
	return l
}

func (l *Lifted) TwoElement() *TwoElementLattice {
	return l.Lattice.TwoElement()
}

func (l *Lifted) Flat() *FlatLattice {
	return l.Lattice.Flat()
}

func (l *Lifted) FlatFinite() *FlatFiniteLattice {
	return l.Lattice.FlatFinite()
}

func (l *Lifted) FlatInt() *FlatIntLattice {
	return l.Lattice.FlatInt()
}

func (l *Lifted) Interval() *IntervalLattice {
	return l.Lattice.Interval()
}

func (l *Lifted) Powerset() *Powerset {
	return l.Lattice.Powerset()
}

func (l *Lifted) Product() *ProductLattice {
	return l.Lattice.Product()
}

func (l *Lifted) Bot() Element {
	if l.bot == nil {
		l.bot = new(LiftedBot)
		l.bot.lattice = l
	}
	return l.bot
}

func (l1 *Lifted) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	// Compare underlying lattices
	case *Lifted:
		return l1.Lattice.Eq(l2.Lattice)
	// Compare underlying lattice of l1 with l2.
	// Makes comparisons more ammenable when used between
	// lifted lattices and their un-lifted variants.
	default:
		return l1.Lattice.Eq(l2)
	}
}

func (l *Lifted) String() string {
	return colorize.LatticeCon("ℒ") + "(" + l.Lattice.String() + ")"
}
