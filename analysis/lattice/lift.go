package lattice

// Lifted is a lattice that was obtained by applying the Lift combinator, 𝓛, to another lattice.
type Lifted struct {
	Lattice
	// index indicates how many times this lattice has been lifted. The first lifting index is 0.
	index int
	bot   *LiftedBot
}

// Lift is a lattice combinator, 𝓛, for lifting any lattice L by introducing
// an additional ⊥ element such thats ⊥ ⊑ x, ∀ x ∈ 𝓛(L).
func Lift(lat Lattice) *Lifted {
	var getIndex func(Lattice) int
	// For a multi-dropped/lifted lattice, retrieve the outermost (least) synthetic ⊥.
	// Construct the new synthetic ⊥ as smaller than that one.
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

// Dropped accesses any dropped lattice underneath the lifted lattice.
func (l *Lifted) Dropped() *Dropped {
	return l.Lattice.Dropped()
}

// Preheight computes how many times a lattice has been lifted.
func (l *Lifted) Preheight() int {
	return 1 + l.Lattice.Preheight()
}

// Lifted is the identity function for the lifted lattice.
func (l *Lifted) Lifted() *Lifted {
	return l
}

// TwoElement converts the underlying lattice to the two-element lattice.
func (l *Lifted) TwoElement() *TwoElementLattice {
	return l.Lattice.TwoElement()
}

// Flat converts the underlying lattice to a flat lattice.
func (l *Lifted) Flat() *FlatLattice {
	return l.Lattice.Flat()
}

// FlatFinite converts the underlying lattice to a finite flat lattice.
func (l *Lifted) FlatFinite() *FlatFiniteLattice {
	return l.Lattice.FlatFinite()
}

// FlatInt converts the underlying lattice to the flat lattice of integers.
func (l *Lifted) FlatInt() *FlatIntLattice {
	return l.Lattice.FlatInt()
}

// Interval converts the underlying lattice to the interval lattice.
func (l *Lifted) Interval() *IntervalLattice {
	return l.Lattice.Interval()
}

// Powerset converts the underlying lattice to a powerset lattice.
func (l *Lifted) Powerset() *Powerset {
	return l.Lattice.Powerset()
}

// Product converts the underlying lattice to a product lattice.
func (l *Lifted) Product() *ProductLattice {
	return l.Lattice.Product()
}

// Bot returns the synthetic ⊥ of the lifted lattice.
func (l *Lifted) Bot() Element {
	if l.bot == nil {
		l.bot = new(LiftedBot)
		l.bot.lattice = l
	}
	return l.bot
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (l1 *Lifted) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	// Compare underlying lattices
	case *Lifted:
		return l1.Lattice.Eq(l2.Lattice)
	// Compare underlying lattice of l1 with l2. Makes comparisons more
	// ammenable when used between lifted lattices and their un-lifted variants.
	default:
		return l1.Lattice.Eq(l2)
	}
}

func (l *Lifted) String() string {
	return colorize.LatticeCon("ℒ") + "(" + l.Lattice.String() + ")"
}
