package lattice

import (
	"errors"
	"fmt"

	"github.com/cs-au-dk/goat/utils"

	"github.com/fatih/color"
)

var opts = utils.Opts()

// colorize exposes consistent colorization strategies for lattices and their elements.
var colorize = struct {
	Lattice    func(...interface{}) string
	LatticeCon func(...interface{}) string
	Element    func(...interface{}) string
	Const      func(...interface{}) string
	Key        func(...interface{}) string
	Attr       func(...interface{}) string
	Field      func(...interface{}) string
}{
	Lattice: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiBlue).SprintFunc())(is...)
	},
	LatticeCon: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgMagenta).SprintFunc())(is...)
	},
	Element: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgCyan).SprintFunc())(is...)
	},
	Const: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiWhite).SprintFunc())(is...)
	},
	Key: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgYellow).SprintFunc())(is...)
	},
	Attr: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiRed).SprintFunc())(is...)
	},
	Field: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgGreen).SprintFunc())(is...)
	},
}

// Common errors.
var (
	errUnsupportedTypeConversion = errors.New("UnsupportedTypeConversion")
	errUnsupportedOperation      = errors.New("UnsupportedOperationError")
	errNotImplemented            = errors.New("NotImplementedError")
	errInternal                  = errors.New("internal error")
	errPatternMatch              = func(v interface{}) error {
		return fmt.Errorf("invalid pattern match: %v %T", v, v)
	}
)

// Element is an interface to which every lattice member implementation must conform.
type Element interface {
	// Lattice element type conversion API
	OneElement() oneElementLatticeElement
	TwoElement() twoElementLatticeElement
	AbstractValue() AbstractValue
	AnalysisState() AnalysisState
	Analysis() Analysis
	AnalysisIntraprocess() AnalysisIntraprocess
	ChannelInfo() ChannelInfo
	Charges() Charges
	Dropped() *DroppedTop
	Flat() FlatElement
	FlatInt() FlatIntElement
	Interval() Interval
	Lifted() *LiftedBot
	Memory() Memory
	OpOutcomes() OpOutcomes
	PointsTo() PointsTo
	Product() Product
	RWMutex() RWMutex
	Cond() Cond
	Set() Set
	ThreadCharges() ThreadCharges

	// Lattice retrieves the lattice to which the element belongs.
	Lattice() Lattice

	// External API for lattice element operations.
	// They dynamically perform lattice type checking.
	Leq(Element) bool
	Geq(Element) bool
	Eq(Element) bool
	Join(Element) Element
	Meet(Element) Element

	// Internal lattice element operations, that skip
	// lattice type checking. Only to be used under the
	// assumption of lattice type safety.
	leq(Element) bool
	geq(Element) bool
	eq(Element) bool
	join(Element) Element
	meet(Element) Element

	// Representational components
	String() string
	// Encodes the distance from the bottom of the lattice
	// to the element that calls this method.
	Height() int
}

// element is the baseline type that all lattice elements must embed.
// It stores which lattice the element belongs to.
type element struct {
	lattice Lattice
}

// Lattice retrieves the lattice that the element belongs to.
func (e element) Lattice() Lattice {
	return e.lattice
}

// AnalysisIntraprocess will panic, as this lattice element does not belong to the analysis result lattice.
func (element) Analysis() Analysis {
	panic(errUnsupportedTypeConversion)
}

// AnalysisIntraprocess will panic, as this lattice element does not belong to the intra-processual analysis result lattice.
func (element) AnalysisIntraprocess() AnalysisIntraprocess {
	panic(errUnsupportedTypeConversion)
}

// AbstractValue will panic, as this lattice element does not belong to the abstract value lattice.
func (e element) AbstractValue() AbstractValue {
	panic(errUnsupportedTypeConversion)
}

// ChannelInfo will panic, as this lattice element does not belong to the abstract channel lattice.
func (element) ChannelInfo() ChannelInfo {
	panic(errUnsupportedTypeConversion)
}

func (element) Dropped() *DroppedTop {
	panic(errUnsupportedTypeConversion)
}

// OpOutcomes will fail, as this lattice element does not belong to the lattice of operation outcomes.
func (element) OpOutcomes() OpOutcomes {
	panic(errUnsupportedTypeConversion)
}

// Flat will fail, as this lattice element does not belong to the flat lattice.
func (element) Flat() FlatElement {
	panic(errUnsupportedTypeConversion)
}

// FlatInt will fail, as this lattice element does not belong to the flat lattice of integers.
func (element) FlatInt() FlatIntElement {
	panic(errUnsupportedTypeConversion)
}

// Interval will fail, as this lattice element does not belong to the interval lattice.
func (element) Interval() Interval {
	panic(errUnsupportedTypeConversion)
}

// Lifted will fail, as this lattice element does not belong to a lifted lattice.
func (element) Lifted() *LiftedBot {
	panic(errUnsupportedTypeConversion)
}

// Memory will fail, as this lattice element does not belong to the abstract memory lattice.
func (element) Memory() Memory {
	panic(errUnsupportedTypeConversion)
}

// OneElement will fail, as this lattice element does not belong to the one-element lattice.
func (element) OneElement() oneElementLatticeElement {
	panic(errUnsupportedTypeConversion)
}

// PointsTo will fail, as this lattice element does not belong to the lattice of points-to sets.
func (element) PointsTo() PointsTo {
	panic(errUnsupportedTypeConversion)
}

// Product will fail, as this lattice element does not belong to a product lattice.
func (element) Product() Product {
	panic(errUnsupportedTypeConversion)
}

// RWMutex will fail, as this lattice element does not belong to the abstract RWMutex lattice.
func (element) RWMutex() RWMutex {
	panic(errUnsupportedTypeConversion)
}

func (element) Cond() Cond {
	panic(errUnsupportedTypeConversion)
}

func (element) Set() Set {
	panic(errUnsupportedTypeConversion)
}

func (element) TwoElement() twoElementLatticeElement {
	panic(errUnsupportedTypeConversion)
}

func (element) Charges() Charges {
	panic(errUnsupportedTypeConversion)
}

func (element) ThreadCharges() ThreadCharges {
	panic(errUnsupportedTypeConversion)
}

func (element) AnalysisState() AnalysisState {
	panic(errUnsupportedTypeConversion)
}

func (element) Height() int {
	panic(errUnsupportedOperation)
}
