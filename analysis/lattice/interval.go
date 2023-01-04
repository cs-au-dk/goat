package lattice

import (
	"fmt"
	"math"
	"strconv"
)

// Interval is an interval and a member of the interval lattice.
// Any interval consists two interval bounds, `low` and `high`.
type Interval struct {
	element
	low  IntervalBound
	high IntervalBound
}

// Interval creates an interval with possibly infinite bounds.
func (elementFactory) Interval(low IntervalBound, high IntervalBound) Interval {
	return Interval{low: low, high: high}
}

// IntervalFinite creates an interval with finite bounds.
func (elementFactory) IntervalFinite(low int, high int) Interval {
	return Interval{
		low:  FiniteBound(low),
		high: FiniteBound(high),
	}
}

// Lattice retrieves the interval lattice for any interval.
func (Interval) Lattice() Lattice {
	return intervalLattice
}

func (e Interval) String() string {
	_, low := e.low.(PlusInfinity)
	_, high := e.high.(MinusInfinity)
	if low && high {
		return "⊥"
	}
	return "[" + e.low.String() + ", " + e.high.String() + "]"
}

// Height returns the height of the interval in the interval lattice.
// The height is computed as the difference between the high and low bounds,
// if both are finite, or -1 otherwise:
//
//	[c1, c2] = c2 - c1, if c1, c2 ∈ ℤ
//	[c1, c2] = -1, if c1 = ±∞  v  c2 = ±∞
func (e Interval) Height() int {
	// Compromise: unknown intervals are represented as height -1
	l, lok := e.low.(FiniteBound)
	h, hok := e.high.(FiniteBound)
	if !(lok && hok) {
		return -1
	}
	return int(math.Max(0, float64(h-l)))
}

// Interval safely converts an interval.
func (e Interval) Interval() Interval {
	return e
}

// IsBot checks that the interval is equal to ⊥ = [∞, -∞].
func (e Interval) IsBot() bool {
	return e == intervalLattice.Bot().Interval()
}

// IsBot checks that the interval is equal to ⊥ = [-∞, ∞].
func (e Interval) IsTop() bool {
	return e == intervalLattice.Top().Interval()
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (e1 Interval) Eq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes m = o.
func (e1 Interval) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (e1 Interval) Leq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes m ⊑ o.
func (e1 Interval) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case Interval:
		return e1.low.Geq(e2.low) && e1.high.Leq(e2.high)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	}
	panic(errInternal)
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (e1 Interval) Geq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes m ⊒ o.
func (e1 Interval) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case Interval:
		return e1.low.Leq(e2.low) && e1.high.Geq(e2.high)
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	}
	panic(errInternal)
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (e1 Interval) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes m ⊔ o.
// The resulting interval takes the lowest of the lower bounds,
// and the highest of the upper bounds.
func (e1 Interval) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case Interval:
		var low, high IntervalBound
		if e1.low.Leq(e2.low) {
			low = e1.low
		} else {
			low = e2.low
		}
		if e1.high.Geq(e2.high) {
			high = e1.high
		} else {
			high = e2.high
		}
		return Interval{low: low, high: high}
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	}
	panic(errInternal)
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (e1 Interval) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes m ⊓ o.
func (e1 Interval) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case Interval:
		// [l1, h1], [l2, h2]:
		switch {
		// h1 < l2 | h2 < l1 => [∞, -∞]
		case e1.high.Lt(e2.low) || e2.high.Lt(e1.low):
			return e1.Lattice().Bot()
		// l1 <= l2 <= h1 <= h2
		case e2.low.Geq(e1.low) && e2.high.Geq(e1.high):
			return Interval{low: e2.low, high: e1.high}
		// l1 <= l2 <= h2 <= h1
		case e2.low.Geq(e1.low) && e1.high.Geq(e2.high):
			return Interval{low: e2.low, high: e2.high}
		// l2 <= l1 <= h1 <= h2
		case e1.low.Geq(e2.low) && e2.high.Geq(e1.high):
			return Interval{low: e1.low, high: e1.high}
		// l2 <= l1 <= h2 <= h1
		case e1.low.Geq(e2.low) && e1.high.Geq(e2.high):
			return Interval{low: e1.low, high: e2.high}
		}
		panic(errInternal)
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	}
	panic(errInternal)
}

// GetFiniteBounds unpacks the interval bounds, if finite, and panics otherwise.
func (i Interval) GetFiniteBounds() (int, int) {
	if i.low.IsInfinite() || i.high.IsInfinite() {
		panic(fmt.Sprintf("Interval %s does not have finite bounds", i))
	}
	return (int)(i.low.(FiniteBound)), (int)(i.high.(FiniteBound))
}

// Low return the lower bound as an integer, if finite, and panics otherwise.
func (i Interval) Low() int {
	if i.low.IsInfinite() {
		panic(fmt.Sprintf("Interval %s does not have finite lower bound", i))
	}
	return (int)(i.low.(FiniteBound))
}

// High returns the upper bound as an integer, if finite, and panics otherwise.
func (i Interval) High() int {
	if i.high.IsInfinite() {
		panic(fmt.Sprintf("Interval %s does not have finite upper bound", i))
	}
	return (int)(i.high.(FiniteBound))
}

// IntervalBound is an interface implemented by all interval lattice bounds i.e.,
// any FiniteBound value, PlusInfinity and MinusInfinity.
type IntervalBound interface {
	String() string

	// IsInfinite checks whether the interval bound is finite.
	IsInfinite() bool

	// BINARY RELATIONS

	// Eq checks for interval bound equality.
	Eq(IntervalBound) bool
	// Leq computes b1 ≤ b2. The semantics is -∞ ≤ c ≤ ∞, where c ∈ ℤ.
	Leq(IntervalBound) bool
	// Geq computes b1 ≥ b2. The semantics is ∞ ≥ c ≥ -∞, where c ∈ ℤ.
	Geq(IntervalBound) bool
	// Lt computes b1 < b2. The semantics is -∞ < c < ∞, where c ∈ ℤ.
	Lt(IntervalBound) bool
	// Gt computes b1 < b2. The semantics is -∞ < c < ∞, where c ∈ ℤ.
	Gt(IntervalBound) bool

	// BINARY OPERATIONS

	// Plus computes b1 + b2. The semantics of plus is:
	//	.-----------------------------.
	// 	|   b1   |   b2   |  b1 ⨣ b2  |
	// 	|========|========|===========|
	// 	|  ∈  ℤ  |  ∈  ℤ  |  b1 + b2  |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ  |    ∞   |     ∞     |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ  |   -∞   |    -∞     |
	// 	|--------|--------|-----------|
	// 	|   -∞   |   -∞   |    -∞     |
	// 	|--------|--------|-----------|
	// 	|    ∞   |    ∞   |     ∞     |
	// 	|--------|--------|-----------|
	// 	|    ∞   |   -∞   |   panic   |
	// 	 -----------------------------
	Plus(IntervalBound) IntervalBound

	// Minus computes b1 - b2. The semantics of minus is:
	//	.-----------------------------.
	// 	|   b1   |   b2   |  b1 - b2  |
	// 	|========|========|===========|
	// 	|  ∈  ℤ  |  ∈  ℤ  |  b1 - b2  |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ  |    ∞   |    -∞     |
	// 	|--------|--------|-----------|
	// 	|    ∞   |  ∈  ℤ  |     ∞     |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ  |   -∞   |     ∞     |
	// 	|--------|--------|-----------|
	// 	|   -∞   |  ∈  ℤ  |    -∞     |
	// 	|--------|--------|-----------|
	// 	|   -∞   |   -∞   |   panic   |
	// 	|--------|--------|-----------|
	// 	|    ∞   |   -∞   |     ∞     |
	// 	|--------|--------|-----------|
	// 	|   -∞   |    ∞   |    -∞     |
	// 	|--------|--------|-----------|
	// 	|    ∞   |    ∞   |   panic   |
	// 	 -----------------------------
	Minus(IntervalBound) IntervalBound

	// Mult computes b1 * b2. The semantics of multiplication is:
	//	.-----------------------------.
	// 	|   b1   |   b2   |  b1 * b2  |
	// 	|========|========|===========|
	// 	|  ∈  ℤ  |  ∈  ℤ  |  b1 * b2  |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ+ |    ∞   |     ∞     |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ+ |   -∞   |    -∞     |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ- |   -∞   |     ∞     |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ- |    ∞   |    -∞     |
	// 	|--------|--------|-----------|
	// 	|    ∞   |    ∞   |     ∞     |
	// 	|--------|--------|-----------|
	// 	|   -∞   |   -∞   |     ∞     |
	// 	|--------|--------|-----------|
	// 	|    ∞   |   -∞   |   panic   |
	// 	|--------|--------|-----------|
	// 	|  (-)∞  |    0   |   panic   |
	// 	 -----------------------------
	Mult(IntervalBound) IntervalBound

	// Div computes b1 / b2. The semantics of division is:
	//	.-----------------------------.
	// 	|   b1   |   b2   |  b1 / b2  |
	// 	|========|========|===========|
	// 	|  ∈ ℤ≠0 |  ∈ ℤ≠0 |  b1 / b2  |
	// 	|--------|--------|-----------|
	// 	|   -∞   |  ∈ ℤ≠0 |     -∞    |
	// 	|--------|--------|-----------|
	// 	|    ∞   |  ∈ ℤ≠0 |      ∞    |
	// 	|--------|--------|-----------|
	// 	|  ∈  ℤ  |  (-)∞  |     0     |
	// 	|--------|--------|-----------|
	// 	|  (-)∞  |  (-)∞  |   panic   |
	// 	|--------|--------|-----------|
	// 	|  ∀ b1  |    0   |   panic   |
	// 	 -----------------------------
	Div(IntervalBound) IntervalBound

	// Max computes max(b1, b2). The semantics of maximum is:
	//	.--------------------------------.
	// 	|   b1   |   b2   | max(b1, b2)  |
	// 	|========|========|==============|
	// 	|  ∈  ℤ  |  ∈  ℤ  | max(b1, b2)  |
	// 	|--------|--------|--------------|
	// 	|  ∀ b1  |    ∞   |       ∞      |
	// 	 --------------------------------
	Max(IntervalBound) IntervalBound

	// Min computes min(b1, b2). The semantics of minimum is:
	//	.--------------------------------.
	// 	|   b1   |   b2   | min(b1, b2)  |
	// 	|========|========|==============|
	// 	|  ∈  ℤ  |  ∈  ℤ  | min(b1, b2)  |
	// 	|--------|--------|--------------|
	// 	|  ∀ b1  |   -∞   |      -∞      |
	// 	 --------------------------------
	Min(IntervalBound) IntervalBound
}

type (
	// FiniteBound is used to represent finite limits of an interval value.
	FiniteBound int
	// PlusInfinity represents ∞.
	PlusInfinity struct{}
	// MinusInfinity represents -∞.
	MinusInfinity struct{}
)

// IsInfinite is false for the finite bound.
func (FiniteBound) IsInfinite() bool {
	return false
}

func (b FiniteBound) String() string {
	return colorize.Element(strconv.Itoa((int)(b)))
}

// Eq compares for equality with another bound. Two finite bounds
// are equal if their underlying values are equal.
func (b1 FiniteBound) Eq(b2 IntervalBound) bool {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 == b2
	}
	return false
}

// Leq computes b1 ≤ b2. The semantics is -∞ ≤ c ≤ ∞, where c ∈ ℤ.
func (b1 FiniteBound) Leq(b2 IntervalBound) bool {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 <= b2
	case PlusInfinity:
		return true
	case MinusInfinity:
		return false
	}
	return false
}

// Geq computes b1 ≥ b2. The semantics is ∞ ≥ c ≥ -∞, where c ∈ ℤ.
func (b1 FiniteBound) Geq(b2 IntervalBound) bool {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 >= b2
	case PlusInfinity:
		return false
	case MinusInfinity:
		return true
	}
	return false
}

// Lt computes b1 < b2. The semantics is -∞ < c < ∞, where c ∈ ℤ.
func (b1 FiniteBound) Lt(b2 IntervalBound) bool {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 < b2
	case PlusInfinity:
		return true
	case MinusInfinity:
		return false
	}
	return false
}

// Gt computes b1 < b2. The semantics is -∞ < c < ∞, where c ∈ ℤ.
func (b1 FiniteBound) Gt(b2 IntervalBound) bool {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 > b2
	case PlusInfinity:
		return false
	case MinusInfinity:
		return true
	}
	return false
}

// Plus computes b1 + b2. The semantics of plus is:
//
//	.--------------------.
//	|   b2   |  b1 + b2  |
//	|========|===========|
//	|   ∈ ℤ  |  b1 + b2  |
//	|--------|-----------|
//	|    ∞   |     ∞     |
//	|--------|-----------|
//	|   -∞   |    -∞     |
//	 --------------------
func (b1 FiniteBound) Plus(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 + b2
	case PlusInfinity:
		return PlusInfinity{}
	case MinusInfinity:
		return MinusInfinity{}
	}
	return nil
}

// Plus computes b1 - b2. The semantics of plus is:
//
//	.--------------------.
//	|   b2   |  b1 - b2  |
//	|========|===========|
//	|   ∈ ℤ  |  b1 - b2  |
//	|--------|-----------|
//	|    ∞   |    -∞     |
//	|--------|-----------|
//	|   -∞   |     ∞     |
//	 --------------------
func (b1 FiniteBound) Minus(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 - b2
	case PlusInfinity:
		return MinusInfinity{}
	case MinusInfinity:
		return PlusInfinity{}
	}
	return nil
}

// Mult computes b1 * b2. The semantics of multiplication is:
//
//	.-----------------------------.
//	|   b1   |   b2   |  b1 * b2  |
//	|========|========|===========|
//	|  ∈  ℤ  |  ∈  ℤ  |  b1 * b2  |
//	|--------|--------|-----------|
//	|  ∈  ℤ+ |    ∞   |     ∞     |
//	|--------|--------|-----------|
//	|  ∈  ℤ+ |   -∞   |    -∞     |
//	|--------|--------|-----------|
//	|  ∈  ℤ- |   -∞   |     ∞     |
//	|--------|--------|-----------|
//	|  ∈  ℤ- |    ∞   |    -∞     |
//	|--------|--------|-----------|
//	|    0   |  (-)∞  |   panic   |
//	 -----------------------------
func (b1 FiniteBound) Mult(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 * b2
	case PlusInfinity:
		switch {
		case b1 > 0:
			return PlusInfinity{}
		case b1 == 0:
			panic("0 * ∞")
		case b1 < 0:
			return MinusInfinity{}
		}
	case MinusInfinity:
		switch {
		case b1 > 0:
			return MinusInfinity{}
		case b1 == 0:
			panic("0 * -∞")
		case b1 < 0:
			return PlusInfinity{}
		}
	}
	return nil
}

// Div computes b1 / b2. The semantics of division is:
//
//	.-----------------------------.
//	|   b1   |   b2   |  b1 / b2  |
//	|========|========|===========|
//	|  ∈ ℤ≠0 |  ∈ ℤ≠0 |  b1 / b2  |
//	|--------|--------|-----------|
//	|  ∈  ℤ  |  (-)∞  |     0     |
//	|--------|--------|-----------|
//	|  ∀ b1  |    0   |   panic   |
//	 -----------------------------
func (b1 FiniteBound) Div(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		switch {
		case b2 == 0 && b1 > 0:
			return PlusInfinity{}
		case b2 == 0 && b1 < 0:
			return MinusInfinity{}
		case b1 == 0 && b2 == 0:
			panic("0 / 0")
		}
		return b1 / b2
	case PlusInfinity:
		return FiniteBound(0)
	case MinusInfinity:
		return FiniteBound(0)
	}
	return nil
}

// Max computes max(b1, b2). The semantics of maximum is:
//
//	.-----------------------.
//	|   b2   | max(b1, b2)  |
//	|========|==============|
//	|  ∈  ℤ  | max(b1, b2)  |
//	|--------|--------------|
//	|   -∞   |      b1      |
//	|--------|--------------|
//	|    ∞   |      ∞       |
//	 -----------------------
func (b1 FiniteBound) Max(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		if b1 < b2 {
			return b2
		}
		return b1
	case PlusInfinity:
		return b2
	case MinusInfinity:
		return b1
	}
	return nil
}

// Min computes min(b1, b2). The semantics of maximum is:
//
//	.-----------------------.
//	|   b2   | min(b1, b2)  |
//	|========|==============|
//	|  ∈  ℤ  | min(b1, b2)  |
//	|--------|--------------|
//	|   -∞   |     -∞       |
//	|--------|--------------|
//	|    ∞   |      b1      |
//	 -----------------------
func (b1 FiniteBound) Min(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		if b1 < b2 {
			return b1
		}
		return b2
	case PlusInfinity:
		return b1
	case MinusInfinity:
		return b2
	}
	return nil
}

// IsInfinite is true for ∞.
func (PlusInfinity) IsInfinite() bool {
	return true
}

func (PlusInfinity) String() string {
	return colorize.Element("∞")
}

// Eq checks for interval bound equality.
func (PlusInfinity) Eq(b2 IntervalBound) bool {
	switch b2.(type) {
	case PlusInfinity:
		return true
	}
	return false
}

// Leq computes ∞ ≤ b.
func (PlusInfinity) Leq(b2 IntervalBound) bool {
	switch b2.(type) {
	case PlusInfinity:
		return true
	}
	return false
}

// Geq computes ∞ ≥ b. It is always true as ∞ is the largest possible bound.
func (PlusInfinity) Geq(IntervalBound) bool {
	return true
}

// Geq computes ∞ < b. It is always false as ∞ is the largest possible bound.
func (PlusInfinity) Lt(IntervalBound) bool {
	return false
}

// Gt computes ∞ > b.
func (PlusInfinity) Gt(b2 IntervalBound) bool {
	switch b2.(type) {
	case PlusInfinity:
		return false
	}
	return true
}

// Plus computes ∞ + b. The semantics of plus is:
//
//	.---------------------.
//	|    b    |   ∞ + b   |
//	|=========|===========|
//	|   ∈ ℤ   |     ∞     |
//	|---------|-----------|
//	|   -∞    |   panic   |
//	|---------|-----------|
//	|  	 ∞    |     ∞     |
//	 ---------------------
func (PlusInfinity) Plus(b2 IntervalBound) IntervalBound {
	switch b2.(type) {
	case MinusInfinity:
		panic("∞ - ∞")
	}
	return PlusInfinity{}
}

// Minus computes ∞ - b. The semantics of minus is:
//
//	.----------=----------.
//	|    b    |   ∞ - b   |
//	|=========|===========|
//	|  ∈ ℤ≠0  |     ∞     |
//	|---------|-----------|
//	|  	-∞    |     ∞     |
//	|---------|-----------|
//	|  	 ∞    |    -∞     |
//	|---------|-----------|
//	|    0    |   panic   |
//	 ---------------------
func (PlusInfinity) Minus(b2 IntervalBound) IntervalBound {
	switch b2.(type) {
	case PlusInfinity:
		panic("∞ - ∞")
	}
	return PlusInfinity{}
}

// Mult computes ∞ * b. The semantics of minus is:
//
//	.----------=----------.
//	|    b    |   ∞ * b   |
//	|=========|===========|
//	|   ∈ ℤ+  |     ∞     |
//	|---------|-----------|
//	|   ∈ ℤ-  |    -∞     |
//	|---------|-----------|
//	|  	 ∞    |     ∞     |
//	|---------|-----------|
//	|  0, -∞  |   panic   |
//	 ---------------------
func (PlusInfinity) Mult(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		switch {
		case b2 < 0:
			return MinusInfinity{}
		case b2 == 0:
			panic("∞ * 0")
		}
	case MinusInfinity:
		panic("∞ * -∞")
	}
	return PlusInfinity{}
}

// Div computes ∞ / b. The semantics of division is:
//
//	.--------------------.
//	|    b   |   ∞ / b   |
//	|========|===========|
//	|  ∈ ℤ≠0 |     0     |
//	|--------|-----------|
//	|  (-)∞  |   panic   |
//	|--------|-----------|
//	|    0   |   panic   |
//	 --------------------
func (PlusInfinity) Div(b2 IntervalBound) IntervalBound {
	switch b2.(type) {
	case PlusInfinity:
		panic("∞ / ∞")
	case MinusInfinity:
		panic("∞ / -∞")
	}
	return PlusInfinity{}
}

// Max computes max(∞, b). The semantics of maximum is:
//
//	.----------------------.
//	|   b   |   max(∞, b)  |
//	|=======|==============|
//	|  ∀ b  |      ∞       |
//	 ----------------------
func (PlusInfinity) Max(IntervalBound) IntervalBound {
	return PlusInfinity{}
}

// Min computes min(∞, b). The semantics of minimum is:
//
//	.----------------------.
//	|   b   |   min(∞, b)  |
//	|=======|==============|
//	|  ∀ b  |      b       |
//	 ----------------------
func (PlusInfinity) Min(b2 IntervalBound) IntervalBound {
	return b2
}

// IsInfinite is true for -∞.
func (MinusInfinity) IsInfinite() bool {
	return true
}

func (MinusInfinity) String() string {
	return colorize.Element("-∞")
}

// Eq computes -∞ = b.
func (MinusInfinity) Eq(b2 IntervalBound) bool {
	switch b2.(type) {
	case MinusInfinity:
		return true
	}
	return false
}

// Leq computes -∞ ≤ b. It is always true as -∞ is the smallest possible bound.
func (MinusInfinity) Leq(IntervalBound) bool {
	return true
}

// Geq computes -∞ ≥ b.
func (MinusInfinity) Geq(b2 IntervalBound) bool {
	switch b2.(type) {
	case MinusInfinity:
		return true
	}
	return false
}

// Lt computes -∞ < b.
func (MinusInfinity) Lt(b2 IntervalBound) bool {
	switch b2.(type) {
	case MinusInfinity:
		return false
	}
	return true
}

// Gt computes -∞ > b. It is always false as -∞ is the smallest possible bound.
func (MinusInfinity) Gt(IntervalBound) bool {
	return false
}

// Plus computes -∞ + b. The semantics of plus is:
//
//	.---------------------.
//	|    b    |  -∞ + b   |
//	|=========|===========|
//	|   ∈ ℤ   |    -∞     |
//	|---------|-----------|
//	|   -∞    |    -∞     |
//	|---------|-----------|
//	|  	 ∞    |   panic   |
//	 ---------------------
func (MinusInfinity) Plus(b IntervalBound) IntervalBound {
	switch b.(type) {
	case PlusInfinity:
		panic("-∞ + ∞")
	}
	return MinusInfinity{}
}

// Minus computes -∞ - b. The semantics of minus is:
//
//	.---------------------.
//	|    b    |  -∞ - b   |
//	|=========|===========|
//	|   ∈ ℤ   |    -∞     |
//	|---------|-----------|
//	|  	-∞    |   panic   |
//	|---------|-----------|
//	|  	 ∞    |     ∞     |
//	 ---------------------
func (MinusInfinity) Minus(b IntervalBound) IntervalBound {
	switch b.(type) {
	case MinusInfinity:
		panic("-∞ - (-∞)")
	}
	return MinusInfinity{}
}

// Mult computes -∞ * b. The semantics of multiplication is:
//
//	.---------------------.
//	|    b    |  -∞ * b   |
//	|=========|===========|
//	|   ∈ ℤ+  |    -∞     |
//	|---------|-----------|
//	|   ∈ ℤ-  |     ∞     |
//	|---------|-----------|
//	|  	-∞    |     ∞     |
//	|---------|-----------|
//	|  0, ∞   |   panic   |
//	 ---------------------
func (MinusInfinity) Mult(b IntervalBound) IntervalBound {
	switch b := b.(type) {
	case FiniteBound:
		switch {
		case b == 0:
			panic("-∞ * 0")
		case b < 0:
			return PlusInfinity{}
		}
	case PlusInfinity:
		panic("-∞ * ∞")
	case MinusInfinity:
		return PlusInfinity{}
	}
	return MinusInfinity{}
}

// Div computes -∞ / b. The semantics of division is:
//
//	.---------------------.
//	|    b    |  -∞ / b   |
//	|=========|===========|
//	|  ∈ ℤ≠0  |    -∞     |
//	|---------|-----------|
//	|   (-)∞  |   panic   |
//	|---------|-----------|
//	|    0    |   panic   |
//	 ---------------------
func (MinusInfinity) Div(b IntervalBound) IntervalBound {
	switch b := b.(type) {
	case FiniteBound:
		if b < 0 {
			return PlusInfinity{}
		}
	case PlusInfinity:
		panic("-∞ / ∞")
	case MinusInfinity:
		panic("-∞ / -∞")
	}
	return MinusInfinity{}
}

// Max computes max(-∞, b). The semantics of maximum is:
//
//	.----------------------.
//	|   b   |  max(-∞, b)  |
//	|=======|==============|
//	|  ∀ b  |      b       |
//	 ----------------------
func (MinusInfinity) Max(b IntervalBound) IntervalBound {
	return b
}

// Min computes min(-∞, b). The semantics of minimum is:
//
//	.----------------------.
//	|   b   |  min(-∞, b)  |
//	|=======|==============|
//	|  ∀ b  |     -∞       |
//	 ----------------------
func (MinusInfinity) Min(b IntervalBound) IntervalBound {
	return MinusInfinity{}
}
