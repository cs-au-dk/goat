package lattice

import (
	"fmt"
)

// Lattice is an interface implemented by all lattice types.
type Lattice interface {
	// Top returns the ⊤ element in a lattice.
	Top() Element
	// Bot returns the ⊥ element in a lattice.
	Bot() Element

	String() string

	// Eq checks whether two lattices represent the same set. Equality checks may be
	// either structural or referential, depending on the decidability of structural
	// equality, or whether referential equality may be more efficient.
	Eq(Lattice) bool

	// Preheight determines how many times a lattice is lifted.
	Preheight() int

	// These methods allow for quick type conversions to concrete lattices from
	// the Lattice interface. Will panic if the lattice is not the correct type.

	// Lifted lattice conversion.
	//
	// Will panic if the lattice is not the correct type.
	Lifted() *Lifted
	// Dropped lattice conversion.
	//
	// Will panic if the lattice is not the correct type.
	Dropped() *Dropped

	// AbstractValue lattice conversion. If the lattice is obtained via lattice combinator e.g., lifted,
	// the type conversion will attempt to retrieve the underlying lattice.
	// Will panic if the lattice is not the correct type.
	AbstractValue() *AbstractValueLattice
	// Analysis lattice conversion. If the lattice is obtained via lattice combinator e.g., lifted,
	// the type conversion will attempt to retrieve the underlying lattice.
	//
	// Will panic if the lattice is not the correct type.
	Analysis() *AnalysisLattice
	// AnalysisIntraprocess lattice conversion. If the lattice is obtained via lattice combinator e.g., lifted,
	// the type conversion will attempt to retrieve the underlying lattice.
	//
	// Will panic if the lattice is not the correct type.
	AnalysisIntraprocess() *AnalysisIntraprocessLattice
	// ChannelInfo lattice conversion. If the lattice is obtained via lattice combinator e.g., lifted,
	// the type conversion will attempt to retrieve the underlying lattice.
	//
	// Will panic if the lattice is not the correct type.
	ChannelInfo() *ChannelInfoLattice
	// Flat lattice conversion. If the lattice is obtained via lattice combinator e.g., lifted,
	// the type conversion will attempt to retrieve the underlying lattice.
	//
	// Will panic if the lattice is not the correct type.
	Flat() *FlatLattice
	// FlatFinite lattice conversion. If the lattice is obtained via lattice combinator e.g., lifted,
	// the type conversion will attempt to retrieve the underlying lattice.
	//
	// Will panic if the lattice is not the correct type.
	FlatFinite() *FlatFiniteLattice
	// FlatInt lattice conversion. If the lattice is obtained via lattice combinator e.g., lifted,
	// the type conversion will attempt to retrieve the underlying lattice.
	//
	// Will panic if the lattice is not the correct type.
	FlatInt() *FlatIntLattice
	Interval() *IntervalLattice
	Memory() *MemoryLattice
	OneElement() *OneElementLattice
	OpOutcomes() *OpOutcomesLattice
	Powerset() *Powerset
	Product() *ProductLattice
	TwoElement() *TwoElementLattice
}

// lattice is the baseline type of all lattices, which must extend this type.
// It implements all methods required by the Lattice interface, but its methods
// typically panic, and must be overriden by concrete lattice implemenations.
type lattice struct{}

// Preheight for the baseline lattice is 0. No concrete implementation of
// a non-lifter/dropped lattice must implement this method.
func (*lattice) Preheight() int {
	return 0
}

// Lifted will panic, as this lattice is not a lifted lattice.
func (*lattice) Lifted() *Lifted {
	panic(errUnsupportedTypeConversion)
}

// Dropped will panic, as this lattice is not a dropped lattice.
func (*lattice) Dropped() *Dropped {
	panic(errUnsupportedTypeConversion)
}

// AbstractValue will panic, as this lattice is not the lattice of abstract values.
func (*lattice) AbstractValue() *AbstractValueLattice {
	panic(errUnsupportedTypeConversion)
}

// Analysis will panic, as this lattice is not the lattice of analysis results.
func (*lattice) Analysis() *AnalysisLattice {
	panic(errUnsupportedTypeConversion)
}

// AnalysisIntraprocess will panic, as this lattice is not the lattice of intra-processual analysis results.
func (*lattice) AnalysisIntraprocess() *AnalysisIntraprocessLattice {
	panic(errUnsupportedTypeConversion)
}

// ChannelInfo will panic, as this lattice is not the lattice of channel values.
func (*lattice) ChannelInfo() *ChannelInfoLattice {
	panic(errUnsupportedTypeConversion)
}

// Flat will panic, as this lattice is not a flat lattice.
func (*lattice) Flat() *FlatLattice {
	panic(errUnsupportedTypeConversion)
}

// FlatFinite will panic, as this lattice is not a finite flat lattice.
func (*lattice) FlatFinite() *FlatFiniteLattice {
	panic(errUnsupportedTypeConversion)
}

// FlatInt will panic, as this lattice is not the flat lattice of integers.
func (*lattice) FlatInt() *FlatIntLattice {
	panic(errUnsupportedTypeConversion)
}

// Interval will panic, as this lattice is not the interval lattice.
func (*lattice) Interval() *IntervalLattice {
	panic(errUnsupportedTypeConversion)
}

// Memory will panic, as this lattice is not the abstract memory lattice.
func (*lattice) Memory() *MemoryLattice {
	panic(errUnsupportedTypeConversion)
}

// OneElement will panic, as this lattice is not the one-element lattice.
func (*lattice) OneElement() *OneElementLattice {
	panic(errUnsupportedTypeConversion)
}

// OpOutcomes will panic, as this lattice is not the lattice of operation outcomes.
func (*lattice) OpOutcomes() *OpOutcomesLattice {
	panic(errUnsupportedTypeConversion)
}

// Powerset will panic, as this lattice is not a powerset lattice.
func (*lattice) Powerset() *Powerset {
	panic(errUnsupportedTypeConversion)
}

// Product will panic, as this lattice is not a product lattice.
func (*lattice) Product() *ProductLattice {
	panic(errUnsupportedTypeConversion)
}

// Product will panic, as this lattice is not the two-element lattice.
func (*lattice) TwoElement() *TwoElementLattice {
	panic(errUnsupportedTypeConversion)
}

// unwrapLattice recursively removes all artificially introduced ⊥ and ⊤
// members, obtained via lifting or dropping a lattice.
func unwrapLattice[T any](l Lattice) T {
	switch l := l.(type) {
	case T:
		return l
	case *Lifted:
		return unwrapLattice[T](l.Lattice)
	case *Dropped:
		return unwrapLattice[T](l.Lattice)
	default:
		panic(errUnsupportedTypeConversion)
	}
}

// checkLatticeMatchThunked ensures that a binary operation is used with elements belonging
// to the same lattices. The string representation of the error is delayed to avoid expensive
// stringification calls.
func checkLatticeMatchThunked(l1, l2 Lattice, thunk func() string) {
	if !l1.Eq(l2) {
		panic(fmt.Sprint(
			"Lattice error - Invalid", thunk(),
			"\nOperand 1 ∈\n",
			l1.String(),
			"\nOperand 2 ∈\n",
			l2.String(),
		))
	}
}

// checkLatticeMatch ensures that a binary operation is used with elements belonging to the
// same lattices.
func checkLatticeMatch(l1, l2 Lattice, binop string) {
	if !l1.Eq(l2) {
		panic(fmt.Sprint(
			"Lattice error - Invalid", binop,
			"\nOperand 1 ∈\n",
			l1.String(),
			"\nOperand 2 ∈\n",
			l2.String(),
		))
	}
}
