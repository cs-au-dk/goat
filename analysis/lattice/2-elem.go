package lattice

// twoElementLatticeElement is the type of members of the two-element lattice
type twoElementLatticeElement bool

var (
	// twoElemBot corresponds to ⊥ in the two-element lattice
	twoElemBot twoElementLatticeElement = false
	// twoElemTop corresponds to ⊤ in the two-element lattice
	twoElemTop twoElementLatticeElement = true
)

// TwoElement creates a two-element lattice member from the given boolean value.
// It generates ⊥ for false, and ⊤ for true.
func (elementFactory) TwoElement(b bool) twoElementLatticeElement {
	return twoElementLatticeElement(b)
}

// Lattice returns the lattice of the two-element lattice members.
func (twoElementLatticeElement) Lattice() Lattice {
	return twoElementLattice
}

// AsBool represents the two-element lattice member as a boolean value.
// It yields ⊥ for false, and ⊤ for true.
func (b twoElementLatticeElement) AsBool() bool {
	return bool(b)
}

func (b twoElementLatticeElement) String() string {
	if b {
		return colorize.Element("⊤")
	}
	return colorize.Element("⊥")
}

// Eq is equivalent to e1 = e2, where e1, e2 ∈ TwoElementLattice.
func (e1 twoElementLatticeElement) Eq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "=")
	return e1.eq(e2)
}

// qq is equivalent to e1 = e2.
func (e1 twoElementLatticeElement) eq(e2 Element) bool {
	return e1 == e2
}

// Geq is equivalent to e1 ⊒ e2, where e1, e2 ∈ TwoElementLattice.
func (e1 twoElementLatticeElement) Geq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq is equivalent to e1 ⊒ e2.
func (e1 twoElementLatticeElement) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case twoElementLatticeElement:
		return (bool)(e1 || !e2)
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

// Leq is equivalent to e1 ⊑ e2, where e1, e2 ∈ TwoElementLattice.
func (e1 twoElementLatticeElement) Leq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq is equivalent to e1 ⊑ e2.
func (e1 twoElementLatticeElement) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case twoElementLatticeElement:
		return (bool)(!e1 || e2)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

// Join is equivalent to e1 ⊔ e2, where e1, e2 ∈ TwoElementLattice.
func (e1 twoElementLatticeElement) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join is equivalent to e1 ⊔ e2.
func (e1 twoElementLatticeElement) join(e2 Element) Element {
	switch e2.(type) {
	case twoElementLatticeElement:
		if e1 {
			return e1
		}
		return e2
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

// Meet is equivalent to e1 ⊓ e2, where e1, e2 ∈ TwoElementLattice.
func (e1 twoElementLatticeElement) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet is equivalent to e1 ⊓ e2.
func (e1 twoElementLatticeElement) meet(e2 Element) Element {
	switch e2.(type) {
	case twoElementLatticeElement:
		if e1 {
			return e2
		}
		return e1
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	default:
		panic(errInternal)
	}
}

// Height returns the height of the element in the lattice: 0 for ⊥, 1 for ⊤.
func (b twoElementLatticeElement) Height() int {
	if bool(b) {
		return 1
	}
	return 0
}

// TwoElement safely converts the two-element lattice element.
func (b twoElementLatticeElement) TwoElement() twoElementLatticeElement {
	return b
}

// AbstractValue will panic for the two-element lattice element.
func (twoElementLatticeElement) AbstractValue() AbstractValue {
	panic(errUnsupportedTypeConversion)
}

// Analysis will panic for the two-element lattice element.
func (twoElementLatticeElement) Analysis() Analysis {
	panic(errUnsupportedTypeConversion)
}

// AnalysisIntraprocess will panic for the two-element lattice element.
func (twoElementLatticeElement) AnalysisIntraprocess() AnalysisIntraprocess {
	panic(errUnsupportedTypeConversion)
}

// AnalysisState will panic for the two-element lattice element.
func (twoElementLatticeElement) AnalysisState() AnalysisState {
	panic(errUnsupportedTypeConversion)
}

// ChannelInfo will panic for the two-element lattice element.
func (twoElementLatticeElement) ChannelInfo() ChannelInfo {
	panic(errUnsupportedTypeConversion)
}

// Cond will panic for the two-element lattice element.
func (twoElementLatticeElement) Cond() Cond {
	panic(errUnsupportedTypeConversion)
}

// Dropped will panic for the two-element lattice element.
func (twoElementLatticeElement) Dropped() *DroppedTop {
	panic(errUnsupportedTypeConversion)
}

// OneElement will panic for the two-element lattice element.
func (twoElementLatticeElement) OneElement() oneElementLatticeElement {
	panic(errUnsupportedTypeConversion)
}

// OpOutcomes will panic for the two-element lattice element.
func (twoElementLatticeElement) OpOutcomes() OpOutcomes {
	panic(errUnsupportedTypeConversion)
}

// Flat will panic for the two-element lattice element.
func (twoElementLatticeElement) Flat() FlatElement {
	panic(errUnsupportedTypeConversion)
}

// FlatIntElement will panic for the two-element lattice element.
func (twoElementLatticeElement) FlatInt() FlatIntElement {
	panic(errUnsupportedTypeConversion)
}

// Interval will panic for the two-element lattice element.
func (twoElementLatticeElement) Interval() Interval {
	panic(errUnsupportedTypeConversion)
}

// Lifted will panic for the two-element lattice element.
func (twoElementLatticeElement) Lifted() *LiftedBot {
	panic(errUnsupportedTypeConversion)
}

// Memory will panic for the two-element lattice element.
func (twoElementLatticeElement) Memory() Memory {
	panic(errUnsupportedTypeConversion)
}

// PointsTo will panic for the two-element lattice element.
func (twoElementLatticeElement) PointsTo() PointsTo {
	panic(errUnsupportedTypeConversion)
}

// Product will panic for the two-element lattice element.
func (twoElementLatticeElement) Product() Product {
	panic(errUnsupportedTypeConversion)
}

// RWMutex will panic for the two-element lattice element.
func (twoElementLatticeElement) RWMutex() RWMutex {
	panic(errUnsupportedTypeConversion)
}

// Set will panic for the two-element lattice element.
func (twoElementLatticeElement) Set() Set {
	panic(errUnsupportedTypeConversion)
}

// Charges will panic for the two-element lattice element.
func (twoElementLatticeElement) Charges() Charges {
	panic(errUnsupportedTypeConversion)
}

// ThreadCharges will panic for the two-element lattice element.
func (twoElementLatticeElement) ThreadCharges() ThreadCharges {
	panic(errUnsupportedTypeConversion)
}
