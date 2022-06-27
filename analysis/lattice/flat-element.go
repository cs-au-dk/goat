package lattice

import (
	"fmt"
	"strconv"
)

type flatElementBase struct {
	element
}

type FlatElement interface {
	Element
	IsBot() bool
	IsTop() bool
	Value() interface{}
	// Check via equality whether the flat element
	// represents the given value. May also be overloaded
	// with flat elements directly to leverage lattice element equality.
	Is(x interface{}) bool
}

type FlatTop struct {
	flatElementBase
}
type FlatBot struct {
	flatElementBase
}

func (flatElementBase) Value() interface{} {
	panic("Called Value() on a FlatBot/Top element")
}

func (f1 flatElementBase) Is(f2 interface{}) bool {
	return f1 == f2
}

func (e FlatBot) Flat() FlatElement {
	return e
}

func (e FlatBot) IsBot() bool {
	return true
}

func (e FlatBot) IsTop() bool {
	return false
}

func (FlatBot) String() string {
	return colorize.Element("⊥")
}

func (FlatBot) Height() int {
	return 0
}

func (e1 FlatBot) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 FlatBot) leq(e2 Element) bool {
	switch e2.(type) {
	case FlatBot:
		return true
	case *LiftedBot:
		return false
	default:
		return true
	}
}

func (e1 FlatBot) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 FlatBot) geq(e2 Element) bool {
	switch e2.(type) {
	case FlatBot:
		return true
	case *LiftedBot:
		return true
	default:
		return false
	}
}

func (e1 FlatBot) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 FlatBot) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

func (e1 FlatBot) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 FlatBot) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case *LiftedBot:
		return e1
	default:
		return e2
	}
}

func (e1 FlatBot) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 FlatBot) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case *LiftedBot:
		return e2
	default:
		return e1
	}
}

func (e FlatTop) Flat() FlatElement {
	return e
}

func (e FlatTop) IsBot() bool {
	return false
}

func (e FlatTop) IsTop() bool {
	return true
}

func (FlatTop) String() string {
	return colorize.Element("T")
}

func (FlatTop) Height() int {
	return 2
}

func (e1 FlatTop) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 FlatTop) leq(e2 Element) bool {
	switch e2.(type) {
	case FlatTop:
		return true
	case *DroppedTop:
		return true
	default:
		return false
	}
}

func (e1 FlatTop) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 FlatTop) geq(e2 Element) bool {
	switch e2.(type) {
	case *DroppedTop:
		return false
	default:
		return true
	}
}

func (e1 FlatTop) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 FlatTop) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

func (e1 FlatTop) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 FlatTop) join(e2 Element) Element {
	switch e2.(type) {
	case *DroppedTop:
		return e2
	default:
		return e1
	}
}

func (e1 FlatTop) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 FlatTop) meet(e2 Element) Element {
	switch e2.(type) {
	case *DroppedTop:
		return e1
	default:
		return e2
	}
}

type flatElement struct {
	element
	value interface{}
}

func (elementFactory) Flat(lat Lattice) func(interface{}) FlatElement {
	switch lat := lat.(type) {
	case *ConstantPropagationLattice:
		return func(v interface{}) FlatElement {
			return flatElement{
				element{lat},
				v,
			}
		}
	case *MutexLattice:
		return func(v interface{}) FlatElement {
			switch v := v.(type) {
			case bool:
				return flatElement{
					element{lat},
					v,
				}
			default:
				panic(fmt.Sprintf("%s is not a Mutex value", v))
			}
		}
	case *FlatFiniteLattice:
		return func(v interface{}) FlatElement {
			if el, ok := lat.dom[v]; ok {
				return el.(flatElement)
			}
			panic(fmt.Sprintf("%s is not part of %s", v, lat))
		}
	case *Lifted:
		return elFact.Flat(lat.Lattice)
	default:
		panic("Attempted creating a flat element with a non-flat lattice")
	}
}

func (elementFactory) Constant(x interface{}) FlatElement {
	return elFact.Flat(constantPropagationLattice)(x)
}

// Retrieve underlying value of the element.
func (e flatElement) Value() interface{} {
	return e.value
}

func (e flatElement) Is(x interface{}) bool {
	switch x := x.(type) {
	case FlatElement:
		return e.Eq(x)
	}
	return e.value == x
}

func (e flatElement) IsBot() bool {
	return false
}

func (e flatElement) IsTop() bool {
	return false
}

func (e flatElement) Flat() FlatElement {
	return e
}

func (e flatElement) String() string {
	return colorize.Element(fmt.Sprintf("%v", e.value))
}

func (e flatElement) Height() int {
	return 1
}

func (e1 flatElement) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 flatElement) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *DroppedTop:
		return true
	case FlatTop:
		return true
	case FlatBot:
		return false
	case FlatElement:
		return e1.value == e2.Value()
	default:
		return false
	}
}

func (e1 flatElement) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 flatElement) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case *DroppedTop:
		return false
	case FlatTop:
		return false
	case FlatBot:
		return true
	case FlatElement:
		return e1.value == e2.Value()
	default:
		return true
	}
}

func (e1 flatElement) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 flatElement) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

func (e1 flatElement) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 flatElement) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case flatElement:
		if e1.value == e2.value {
			return e1
		}
		return e1.lattice.Top()
	case FlatTop:
		return e2
	case *DroppedTop:
		return e2
	default:
		return e1
	}
}

func (e1 flatElement) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 flatElement) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case flatElement:
		if e1.value == e2.value {
			return e1
		}
		return e1.lattice.Bot()
	case FlatTop:
		return e1
	case *DroppedTop:
		return e1
	default:
		return e2
	}
}

type FlatIntElement struct {
	element
	value int
}

func (elementFactory) FlatInt(v int) FlatIntElement {
	return FlatIntElement{
		element{flatIntLattice},
		v,
	}
}

func (e FlatIntElement) IValue() int {
	return e.value
}

// Satisfy the FlatElement interface
func (e FlatIntElement) Value() interface{} {
	return e.value
}

func (e FlatIntElement) Is(x interface{}) bool {
	return e.value == x
}

func (e FlatIntElement) IsBot() bool {
	return false
}

func (e FlatIntElement) IsTop() bool {
	return false
}

func (e FlatIntElement) Flat() FlatElement {
	return e
}

func (e FlatIntElement) FlatInt() FlatIntElement {
	return e
}

func (e FlatIntElement) String() string {
	return colorize.Element(strconv.Itoa(e.value))
}

func (e FlatIntElement) Height() int {
	return 1
}

func (e1 FlatIntElement) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 FlatIntElement) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case FlatTop:
		return true
	case FlatBot:
		return false
	case FlatIntElement:
		return e1.value == e2.value
	case *DroppedTop:
		return true
	default:
		return false
	}
}

func (e1 FlatIntElement) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 FlatIntElement) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case FlatTop:
		return false
	case FlatBot:
		return true
	case FlatIntElement:
		return e1.value == e2.value
	case *DroppedTop:
		return false
	default:
		return true
	}
}

func (e1 FlatIntElement) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 FlatIntElement) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

func (e1 FlatIntElement) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 FlatIntElement) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case FlatIntElement:
		if e1.value == e2.value {
			return e1
		}
		return e1.lattice.Top()
	case FlatTop:
		return e2
	case *DroppedTop:
		return e2
	default:
		return e1
	}
}

func (e1 FlatIntElement) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 FlatIntElement) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case FlatIntElement:
		if e1.value == e2.value {
			return e1
		}
		return e1.lattice.Bot()
	case FlatTop:
		return e1
	case *DroppedTop:
		return e1
	default:
		return e2
	}
}
