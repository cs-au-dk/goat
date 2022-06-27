package lattice

import (
	"fmt"
	"math"
	"strconv"
)

type Interval struct {
	element
	low  IntervalBound
	high IntervalBound
}

// Create interval with possibly infinite bounds.
func (elementFactory) Interval(low IntervalBound, high IntervalBound) Interval {
	return Interval{low: low, high: high}
}

// Create interval with finite bounds.
func (elementFactory) IntervalFinite(low int, high int) Interval {
	return Interval{
		low:  FiniteBound(low),
		high: FiniteBound(high),
	}
}

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

func (e Interval) Height() int {
	// Compromise: unknown intervals are represented as height -1
	l, lok := e.low.(FiniteBound)
	h, hok := e.high.(FiniteBound)
	if !(lok && hok) {
		return -1
	}
	return int(math.Max(0, float64(h-l)))
}

func (e Interval) Interval() Interval {
	return e
}

func (e Interval) IsBot() bool {
	return e == intervalLattice.Bot().Interval()
}

func (e Interval) IsTop() bool {
	return e == intervalLattice.Top().Interval()
}

func (e1 Interval) Eq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 Interval) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

func (e1 Interval) Leq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 Interval) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case Interval:
		return e1.low.Geq(e2.low) && e1.high.Leq(e2.high)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

func (e1 Interval) Geq(e2 Element) bool {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 Interval) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case Interval:
		return e1.low.Leq(e2.low) && e1.high.Geq(e2.high)
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

func (e1 Interval) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

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
	default:
		panic(errInternal)
	}
}

func (e1 Interval) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

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
		default:
			panic(errInternal)
		}
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	default:
		panic(errInternal)
	}
}

// If bounds are finite, retrieve them. Otherwise panic.
func (i Interval) GetFiniteBounds() (int, int) {
	if i.low.IsInfinite() || i.high.IsInfinite() {
		panic(fmt.Sprintf("Interval %s does not have finite bounds", i))
	}
	return (int)(i.low.(FiniteBound)), (int)(i.high.(FiniteBound))
}

// Return the lower bound as an integer, if finite. Otherwise, panic.
func (i Interval) Low() int {
	if i.low.IsInfinite() {
		panic(fmt.Sprintf("Interval %s does not have finite lower bound", i))
	}
	return (int)(i.low.(FiniteBound))
}

// Return the upper bound as an integer, if finite. Otherwise, panic.
func (i Interval) High() int {
	if i.high.IsInfinite() {
		panic(fmt.Sprintf("Interval %s does not have finite upper bound", i))
	}
	return (int)(i.high.(FiniteBound))
}

type IntervalBound interface {
	String() string

	// Predicates
	IsInfinite() bool

	// Binary relations
	Eq(IntervalBound) bool
	Leq(IntervalBound) bool
	Geq(IntervalBound) bool
	Lt(IntervalBound) bool
	Gt(IntervalBound) bool

	// Binary operations
	Plus(IntervalBound) IntervalBound
	Minus(IntervalBound) IntervalBound
	Mult(IntervalBound) IntervalBound
	Div(IntervalBound) IntervalBound
	Max(IntervalBound) IntervalBound
	Min(IntervalBound) IntervalBound
}

type FiniteBound int
type PlusInfinity struct{}
type MinusInfinity struct{}

func (FiniteBound) IsInfinite() bool {
	return false
}

func (b FiniteBound) String() string {
	return colorize.Element(strconv.Itoa((int)(b)))
}

func (b1 FiniteBound) Eq(b2 IntervalBound) bool {
	switch b2 := b2.(type) {
	case FiniteBound:
		return b1 == b2
	default:
		return false
	}
}

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

func (PlusInfinity) IsInfinite() bool {
	return true
}

func (PlusInfinity) String() string {
	return colorize.Element("∞")
}

func (PlusInfinity) Eq(b2 IntervalBound) bool {
	switch b2.(type) {
	case PlusInfinity:
		return true
	}
	return false
}

func (PlusInfinity) Leq(b2 IntervalBound) bool {
	switch b2.(type) {
	case PlusInfinity:
		return true
	}
	return false
}

func (PlusInfinity) Geq(IntervalBound) bool {
	return true
}

func (PlusInfinity) Lt(IntervalBound) bool {
	return false
}

func (PlusInfinity) Gt(b2 IntervalBound) bool {
	switch b2.(type) {
	case PlusInfinity:
		return false
	default:
		return true
	}
}

func (PlusInfinity) Plus(b2 IntervalBound) IntervalBound {
	switch b2.(type) {
	case MinusInfinity:
		panic("∞ - ∞")
	}
	return PlusInfinity{}
}

func (PlusInfinity) Minus(b2 IntervalBound) IntervalBound {
	switch b2.(type) {
	case PlusInfinity:
		panic("∞ - ∞")
	}
	return PlusInfinity{}
}

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

func (PlusInfinity) Div(b2 IntervalBound) IntervalBound {
	switch b2.(type) {
	case PlusInfinity:
		panic("∞ / ∞")
	case MinusInfinity:
		panic("∞ / -∞")
	}
	return PlusInfinity{}
}

func (PlusInfinity) Max(IntervalBound) IntervalBound {
	return PlusInfinity{}
}

func (PlusInfinity) Min(b2 IntervalBound) IntervalBound {
	return b2
}

func (MinusInfinity) IsInfinite() bool {
	return true
}

func (MinusInfinity) String() string {
	return colorize.Element("-∞")
}

func (MinusInfinity) Eq(b2 IntervalBound) bool {
	switch b2.(type) {
	case MinusInfinity:
		return true
	}
	return false
}

func (MinusInfinity) Leq(IntervalBound) bool {
	return true
}

func (MinusInfinity) Geq(b2 IntervalBound) bool {
	switch b2.(type) {
	case MinusInfinity:
		return true
	}
	return false
}

func (MinusInfinity) Lt(b2 IntervalBound) bool {
	switch b2.(type) {
	case MinusInfinity:
		return false
	}
	return true
}

func (MinusInfinity) Gt(IntervalBound) bool {
	return false
}

func (MinusInfinity) Plus(b2 IntervalBound) IntervalBound {
	switch b2.(type) {
	case PlusInfinity:
		panic("-∞ + ∞")
	}
	return MinusInfinity{}
}

func (MinusInfinity) Minus(b2 IntervalBound) IntervalBound {
	switch b2.(type) {
	case MinusInfinity:
		panic("-∞ - (-∞)")
	}
	return MinusInfinity{}
}

func (MinusInfinity) Mult(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		switch {
		case b2 == 0:
			panic("-∞ * 0")
		case b2 < 0:
			return PlusInfinity{}
		}
	case PlusInfinity:
		panic("-∞ * ∞")
	case MinusInfinity:
		return PlusInfinity{}
	}
	return MinusInfinity{}
}

func (MinusInfinity) Div(b2 IntervalBound) IntervalBound {
	switch b2 := b2.(type) {
	case FiniteBound:
		if b2 < 0 {
			return PlusInfinity{}
		}
	case PlusInfinity:
		panic("-∞ / ∞")
	case MinusInfinity:
		panic("-∞ / -∞")
	}
	return MinusInfinity{}
}

func (MinusInfinity) Max(b2 IntervalBound) IntervalBound {
	return b2
}

func (MinusInfinity) Min(b2 IntervalBound) IntervalBound {
	return MinusInfinity{}
}
