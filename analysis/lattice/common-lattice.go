package lattice

import (
	"log"
)

type Lattice interface {
	Top() Element
	Bot() Element

	String() string
	Eq(Lattice) bool
	// Pre-height returns how many times a lattice is lifted.
	Preheight() int

	// These methods allow for quick type conversions.
	// Suitable, if you know what lattice type to expect.
	// If the lattice is lifted/dropped, type conversion
	// will attempt to retrieve the underlying un-lifted/dropped
	// lattice.
	Lifted() *Lifted
	Dropped() *Dropped

	AbstractValue() *AbstractValueLattice
	Analysis() *AnalysisLattice
	AnalysisIntraprocess() *AnalysisIntraprocessLattice
	AnalysisStateStack() *AnalysisStateStackLattice
	ChannelInfo() *ChannelInfoLattice
	Flat() *FlatLattice
	FlatFinite() *FlatFiniteLattice
	FlatInt() *FlatIntLattice
	Interval() *IntervalLattice
	Map() *MapLattice
	InfiniteMap() *InfiniteMapLattice
	Memory() *MemoryLattice
	OneElement() *OneElementLattice
	OpOutcomes() *OpOutcomesLattice
	Powerset() *Powerset
	Product() *ProductLattice
	TwoElement() *TwoElementLattice
}

type lattice struct{}

func (*lattice) Preheight() int {
	return 0
}

func (*lattice) Lifted() *Lifted {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) Dropped() *Dropped {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) AbstractValue() *AbstractValueLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) Analysis() *AnalysisLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) AnalysisIntraprocess() *AnalysisIntraprocessLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) AnalysisStateStack() *AnalysisStateStackLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) ChannelInfo() *ChannelInfoLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) Flat() *FlatLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) FlatFinite() *FlatFiniteLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) FlatInt() *FlatIntLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) Interval() *IntervalLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) Map() *MapLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) InfiniteMap() *InfiniteMapLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) Memory() *MemoryLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) OneElement() *OneElementLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) OpOutcomes() *OpOutcomesLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) Powerset() *Powerset {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) Product() *ProductLattice {
	panic(errUnsupportedTypeConversion)
}

func (*lattice) TwoElement() *TwoElementLattice {
	panic(errUnsupportedTypeConversion)
}

// Allows us to delay expensive stringification calls
func checkLatticeMatchThunked(l1, l2 Lattice, thunk func() string) {
	if !l1.Eq(l2) {
		log.Fatal(
			"Lattice error - Invalid", thunk(),
			"\nOperand 1 ∈\n",
			l1.String(),
			"\nOperand 2 ∈\n",
			l2.String(),
		)
	}
}

func checkLatticeMatch(l1, l2 Lattice, binop string) {
	if !l1.Eq(l2) {
		log.Fatal(
			"Lattice error - Invalid", binop,
			"\nOperand 1 ∈\n",
			l1.String(),
			"\nOperand 2 ∈\n",
			l2.String(),
		)
	}
}
