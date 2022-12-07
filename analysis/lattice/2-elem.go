package lattice

type twoElementLatticeElement bool

var twoElemBot twoElementLatticeElement = false
var twoElemTop twoElementLatticeElement = true

func (elementFactory) TwoElement(b bool) twoElementLatticeElement {
	return twoElementLatticeElement(b)
}

func (twoElementLatticeElement) Lattice() Lattice {
	return twoElementLattice
}

func (b twoElementLatticeElement) AsBool() bool {
	return bool(b)
}

func (b twoElementLatticeElement) String() string {
	if b {
		return colorize.Element("T")
	}
	return colorize.Element("\u22A5")
}

func (e1 twoElementLatticeElement) Eq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 twoElementLatticeElement) eq(e2 Element) bool {
	return e1 == e2
}

func (e1 twoElementLatticeElement) Geq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊒")
	return e1.geq(e2)
}

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

func (e1 twoElementLatticeElement) Leq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊑")
	return e1.leq(e2)
}

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

func (e1 twoElementLatticeElement) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

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

func (e1 twoElementLatticeElement) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

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

func (b twoElementLatticeElement) Height() int {
	if bool(b) {
		return 1
	}
	return 0
}

func (b twoElementLatticeElement) TwoElement() twoElementLatticeElement {
	return b
}

func (twoElementLatticeElement) AbstractValue() AbstractValue {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Analysis() Analysis {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) AnalysisIntraprocess() AnalysisIntraprocess {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) ChannelInfo() ChannelInfo {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Cond() Cond {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Dropped() *DroppedTop {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) OneElement() oneElementLatticeElement {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) OpOutcomes() OpOutcomes {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Flat() FlatElement {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) FlatInt() FlatIntElement {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Interval() Interval {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Lifted() *LiftedBot {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Memory() Memory {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) PointsTo() PointsTo {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Product() Product {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) RWMutex() RWMutex {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Set() Set {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) Charges() Charges {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) ThreadCharges() ThreadCharges {
	panic(errUnsupportedTypeConversion)
}

func (twoElementLatticeElement) AnalysisState() AnalysisState {
	panic(errUnsupportedTypeConversion)
}
