package lattice

import (
	"errors"
	"fmt"

	"github.com/cs-au-dk/goat/utils"

	"github.com/fatih/color"
)

var opts = utils.Opts()

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

var (
	errUnsupportedTypeConversion = errors.New("UnsupportedTypeConversion")
	errUnsupportedOperation      = errors.New("UnsupportedOperationError")
	errNotImplemented            = errors.New("NotImplementedError")
	errInternal                  = errors.New("internal error")
	errPatternMatch              = func(v interface{}) error {
		return fmt.Errorf("invalid pattern match: %v %T", v, v)
	}
)

type Element interface {
	// Type conversion API
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

	Lattice() Lattice

	// External API for lattice element operations.
	// They dynamically perform lattice type checking.
	Leq(Element) bool
	Geq(Element) bool
	Eq(Element) bool
	Join(Element) Element
	Meet(Element) Element

	// Internal lattice element operations, that skip
	// lattice type checking. Only use under the
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

type element struct {
	lattice Lattice
}

func (e element) Lattice() Lattice {
	return e.lattice
}

func (element) Analysis() Analysis {
	panic(errUnsupportedTypeConversion)
}

func (element) AnalysisIntraprocess() AnalysisIntraprocess {
	panic(errUnsupportedTypeConversion)
}

func (e element) AbstractValue() AbstractValue {
	panic(errUnsupportedTypeConversion)
}

func (element) ChannelInfo() ChannelInfo {
	panic(errUnsupportedTypeConversion)
}

func (element) Cond() Cond {
	panic(errUnsupportedTypeConversion)
}

func (element) Dropped() *DroppedTop {
	panic(errUnsupportedTypeConversion)
}

func (element) OpOutcomes() OpOutcomes {
	panic(errUnsupportedTypeConversion)
}

func (element) Flat() FlatElement {
	panic(errUnsupportedTypeConversion)
}

func (element) FlatInt() FlatIntElement {
	panic(errUnsupportedTypeConversion)
}

func (element) Interval() Interval {
	panic(errUnsupportedTypeConversion)
}

func (element) Lifted() *LiftedBot {
	panic(errUnsupportedTypeConversion)
}

func (element) Memory() Memory {
	panic(errUnsupportedTypeConversion)
}

func (element) OneElement() oneElementLatticeElement {
	panic(errUnsupportedTypeConversion)
}

func (element) PointsTo() PointsTo {
	panic(errUnsupportedTypeConversion)
}

func (element) Product() Product {
	panic(errUnsupportedTypeConversion)
}

func (element) RWMutex() RWMutex {
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
