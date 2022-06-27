package lattice

func Drop(lat Lattice) *Dropped {
	var getIndex func(Lattice) int
	// For a multi-dropped/lifted lattice, dig for the index of the
	// surface-most drop.
	getIndex = func(lat Lattice) int {
		switch lat := lat.(type) {
		case *Lifted:
			return getIndex(lat.Lattice)
		case *Dropped:
			return lat.index + 1
		}
		return 0
	}

	dropped := new(Dropped)

	dropped.Lattice = lat
	dropped.index = getIndex(lat)
	return dropped
}

type Dropped struct {
	Lattice
	// An index above 0 signifies a multi-dropped lattice
	index int
	top   *DroppedTop
}

func (l *Dropped) Lifted() *Lifted {
	return l.Lattice.Lifted()
}

func (l *Dropped) Preheight() int {
	return l.Lattice.Preheight()
}

func (l *Dropped) Dropped() *Dropped {
	return l
}

func (l *Dropped) TwoElement() *TwoElementLattice {
	return l.Lattice.TwoElement()
}

func (l *Dropped) Flat() *FlatLattice {
	return l.Lattice.Flat()
}

func (l *Dropped) FlatFinite() *FlatFiniteLattice {
	return l.Lattice.FlatFinite()
}

func (l *Dropped) FlatInt() *FlatIntLattice {
	return l.Lattice.FlatInt()
}

func (l *Dropped) Interval() *IntervalLattice {
	return l.Lattice.Interval()
}

func (l *Dropped) Map() *MapLattice {
	return l.Lattice.Map()
}

func (l *Dropped) InfiniteMap() *InfiniteMapLattice {
	return l.Lattice.InfiniteMap()
}

func (l *Dropped) Powerset() *Powerset {
	return l.Lattice.Powerset()
}

func (l *Dropped) Product() *ProductLattice {
	return l.Lattice.Product()
}

func (l *Dropped) Top() Element {
	if l.top == nil {
		l.top = new(DroppedTop)
		l.top.lattice = l
	}
	return l.top
}

func (l1 *Dropped) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	// Compare underlying lattices
	case *Dropped:
		return l1.Lattice.Eq(l2.Lattice)
	// Compare underlying lattice of l1 with l2.
	// Makes comparisons more ammenable when used between
	// dropped lattices and their un-dropped variants.
	default:
		return l1.Lattice.Eq(l2)
	}
}

func (l *Dropped) String() string {
	return colorize.LatticeCon("𝒟") +
		"(" + l.Lattice.String() + ")"
}
