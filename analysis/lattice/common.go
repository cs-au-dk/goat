package lattice

import (
	"Goat/utils"
	"errors"
	"fmt"
	"strings"

	"github.com/fatih/color"

	"github.com/benbjohnson/immutable"
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
	AnalysisStateStack() AnalysisStateStack
	ChannelInfo() ChannelInfo
	Charges() Charges
	Dropped() *DroppedTop
	Flat() FlatElement
	FlatInt() FlatIntElement
	InfiniteMap() InfiniteMap
	Interval() Interval
	Lifted() *LiftedBot
	Map() Map
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

func (element) AnalysisStateStack() AnalysisStateStack {
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

func (element) Map() Map {
	panic(errUnsupportedTypeConversion)
}

func (element) InfiniteMap() InfiniteMap {
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

type elementList struct {
	*immutable.List
}

func (el elementList) foreach(do func(index int, e Element)) {
	iter := el.Iterator()
	for !iter.Done() {
		index, ep := iter.Next()
		do(index, ep.(Element))
	}
}

func (el elementList) set(i int, e Element) elementList {
	return elementList{el.Set(i, e)}
}

func (el elementList) get(i int) Element {
	return el.Get(i).(Element)
}

func (el elementList) forall(pred func(i int, e Element) bool) bool {
	iter := el.Iterator()
	for !iter.Done() {
		i, ep := iter.Next()
		e := ep.(Element)
		if !pred(i, e) {
			return false
		}
	}
	return true
}

type elementMap struct {
	*immutable.Map
}

func (em elementMap) foreach(do func(key interface{}, e Element)) {
	iter := em.Iterator()
	for !iter.Done() {
		key, e := iter.Next()
		do(key, e.(Element))
	}
}

func (em elementMap) get(key interface{}) (Element, bool) {
	if ep, found := em.Get(key); found {
		return ep.(Element), true
	} else {
		return nil, false
	}
}

func (em elementMap) getUnsafe(key interface{}) Element {
	ep, found := em.get(key)
	if found {
		return ep
	}
	panic(errInternal)
}

func (em elementMap) set(key interface{}, e Element) elementMap {
	return elementMap{em.Set(key, e)}
}

func (em elementMap) forall(pred func(key interface{}, e Element) bool) bool {
	iter := em.Iterator()
	for !iter.Done() {
		k, ep := iter.Next()
		e := ep.(Element)
		if !pred(k, e) {
			return false
		}
	}
	return true
}

func (em elementMap) String() string {
	iter := em.Iterator()
	str := []string{}
	for !iter.Done() {
		k, ep := iter.Next()
		str = append(str, fmt.Sprintf("%s -> %s", k, ep))
	}
	return "Underlying elementMap: [" + strings.Join(str, ", ") + "]"
}
