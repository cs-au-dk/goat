package lattice

type oneElementLatticeElement struct{}

var oneElem oneElementLatticeElement = struct{}{}

func (elementFactory) OneElement() oneElementLatticeElement {
	return oneElem
}

func (oneElementLatticeElement) Lattice() Lattice {
	return oneElementLattice
}

func (b oneElementLatticeElement) String() string {
	return colorize.Element("𝕚")
}

func (e1 oneElementLatticeElement) Eq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 oneElementLatticeElement) eq(e2 Element) bool {
	return e1 == e2
}

func (e1 oneElementLatticeElement) Geq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 oneElementLatticeElement) geq(e2 Element) bool {
	switch e2.(type) {
	case oneElementLatticeElement:
		return true
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

func (e1 oneElementLatticeElement) Leq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 oneElementLatticeElement) leq(e2 Element) bool {
	switch e2.(type) {
	case oneElementLatticeElement:
		return true
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

func (e1 oneElementLatticeElement) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 oneElementLatticeElement) join(e2 Element) Element {
	switch e2.(type) {
	case oneElementLatticeElement:
		return e1
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

func (e1 oneElementLatticeElement) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 oneElementLatticeElement) meet(e2 Element) Element {
	switch e2.(type) {
	case oneElementLatticeElement:
		return e1
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	default:
		panic(errInternal)
	}
}

func (e oneElementLatticeElement) Height() int {
	return 0
}

func (b oneElementLatticeElement) OneElement() oneElementLatticeElement {
	return b
}

func (b oneElementLatticeElement) TwoElement() twoElementLatticeElement {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) AbstractValue() AbstractValue {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Analysis() Analysis {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) AnalysisIntraprocess() AnalysisIntraprocess {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) ChannelInfo() ChannelInfo {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Cond() Cond {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Dropped() *DroppedTop {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) OpOutcomes() OpOutcomes {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Flat() FlatElement {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) FlatInt() FlatIntElement {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Interval() Interval {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Lifted() *LiftedBot {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Memory() Memory {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) PointsTo() PointsTo {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Product() Product {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) RWMutex() RWMutex {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Set() Set {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) Charges() Charges {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) ThreadCharges() ThreadCharges {
	panic(errUnsupportedTypeConversion)
}

func (oneElementLatticeElement) AnalysisState() AnalysisState {
	panic(errUnsupportedTypeConversion)
}
