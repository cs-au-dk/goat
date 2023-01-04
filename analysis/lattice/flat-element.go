package lattice

import (
	"fmt"
	"strconv"
)

type (
	// flatElementBase is the basis for constructing all members of the flat lattice.
	// Is embedded by ⊥, ⊤ and valued members.
	flatElementBase struct {
		element
	}

	// FlatElement is an interface implemented by all members of any flat lattice.
	// It extends the standard lattice element interface with relevant methods.
	FlatElement interface {
		Element
		// IsBot checks whether the flat lattice member is ⊥.
		IsBot() bool
		// IsTop checks whether the flat lattice member is ⊤.
		IsTop() bool
		// Value
		Value() any
		// Is checks via equality (==) whether the flat element represents the given value.
		// May be overloaded with flat lattice members directly to leverage lattice element equality.
		Is(x any) bool
	}

	// FlatTop is the standard type of the flat ⊤ element.
	FlatTop struct {
		flatElementBase
	}

	// FlatTop is the standard type of the flat ⊥ element.
	FlatBot struct {
		flatElementBase
	}

	// flatElement is a valued member of a flat lattice.
	flatElement struct {
		element
		value any
	}

	// FlatIntElement is a non-⊤/⊥ member of the specialized flat lattice of integers.
	FlatIntElement struct {
		element
		value int
	}
)

// Value will panic, and must only be invoked for valued flat lattice members.
func (flatElementBase) Value() any {
	panic("Called Value() on a FlatBot/Top element")
}

// Is checks whether two flat lattice members are structurally identical.
func (f1 flatElementBase) Is(f2 any) bool {
	return f1 == f2
}

// Flat converts the flat ⊥ member to a FlatElement.
func (e FlatBot) Flat() FlatElement {
	return e
}

// IsBot is true for flat ⊥.
func (e FlatBot) IsBot() bool {
	return true
}

// IsTop is false for flat ⊥.
func (e FlatBot) IsTop() bool {
	return false
}

func (FlatBot) String() string {
	return colorize.Element("⊥")
}

// Height is 0 for flat ⊥.
func (FlatBot) Height() int {
	return 0
}

// Leq computes ⊥ ⊑ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatBot) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes ⊥ ⊑ x, where x is a lattice element.
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

// Geq computes ⊥ ⊒ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatBot) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes ⊥ ⊒ x, where x is a lattice element.
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

// Eq computes ⊥ = x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatBot) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes ⊥ = x, where x is a lattice element.
func (e1 FlatBot) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

// Join computes ⊥ ⊔ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatBot) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes ⊥ ⊔ x, where x is a lattice element.
func (e1 FlatBot) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case *LiftedBot:
		return e1
	default:
		return e2
	}
}

// Meet computes ⊥ ⊓ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatBot) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes ⊥ ⊓ x, where x is a lattice element.
func (e1 FlatBot) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case *LiftedBot:
		return e2
	default:
		return e1
	}
}

// Flat converts the flat ⊤ member to a FlatElement.
func (e FlatTop) Flat() FlatElement {
	return e
}

// IsBot is false for flat ⊤.
func (e FlatTop) IsBot() bool {
	return false
}

// IsTop is true for flat ⊤.
func (e FlatTop) IsTop() bool {
	return true
}

func (FlatTop) String() string {
	return colorize.Element("T")
}

// Height is 2 for flat ⊤.
func (FlatTop) Height() int {
	return 2
}

// Leq computes ⊤ ⊑ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatTop) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes ⊤ ⊑ x, where x is a lattice element.
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

// Geq computes ⊤ ⊒ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatTop) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes ⊤ ⊒ x, where x is a lattice element.
func (e1 FlatTop) geq(e2 Element) bool {
	switch e2.(type) {
	case *DroppedTop:
		return false
	default:
		return true
	}
}

// Eq computes ⊤ = x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatTop) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes ⊤ = x, where x is a lattice element.
func (e1 FlatTop) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

// Join computes ⊤ ⊔ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatTop) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes ⊤ ⊔ x, where x is a lattice element.
func (e1 FlatTop) join(e2 Element) Element {
	switch e2.(type) {
	case *DroppedTop:
		return e2
	default:
		return e1
	}
}

// Meet computes ⊤ ⊓ x, where x is a lattice element.
// Performs lattice type checking.
func (e1 FlatTop) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes ⊤ ⊓ x, where x is a lattice element.
func (e1 FlatTop) meet(e2 Element) Element {
	switch e2.(type) {
	case *DroppedTop:
		return e1
	default:
		return e2
	}
}

// Flat yields a factory for generating valued members belonging to the
// given flat lattice. Accepted lattices are the mutex, finite flat, and
// constant propagation lattice.
func (elementFactory) Flat(lat Lattice) func(any) FlatElement {
	switch lat := lat.(type) {
	case *ConstantPropagationLattice:
		return func(v any) FlatElement {
			return flatElement{
				element{lat},
				v,
			}
		}
	case *MutexLattice:
		return func(v any) FlatElement {
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
		return func(v any) FlatElement {
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

// Constant produces a member of the constant propagation lattice.
func (elementFactory) Constant(x any) FlatElement {
	return elFact.Flat(constantPropagationLattice)(x)
}

// Retrieve underlying value of the element.
func (e flatElement) Value() any {
	return e.value
}

// Is checks for equality with the given value.
// If the value is a another flat element, it leverages lattice member equality.
// Otherwise, it compares the given value with the element's underlying value.
func (e flatElement) Is(x any) bool {
	switch x := x.(type) {
	case FlatElement:
		return e.Eq(x)
	}
	return e.value == x
}

// IsBot is false for non-⊥ elements.
func (e flatElement) IsBot() bool {
	return false
}

// IsTop is false for non-⊤ elements.
func (e flatElement) IsTop() bool {
	return false
}

// Flat safely converts to a flat element.
func (e flatElement) Flat() FlatElement {
	return e
}

func (e flatElement) String() string {
	return colorize.Element(fmt.Sprintf("%v", e.value))
}

// Height always returns 1 for known members of flat lattices.
func (e flatElement) Height() int {
	return 1
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (e1 flatElement) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes m ⊑ o.
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

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (e1 flatElement) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes m ⊒ o.
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

// Eq computes m = o. Performs lattice dynamic type checking.
func (e1 flatElement) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes m = o.
func (e1 flatElement) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (e1 flatElement) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes m ⊔ o.
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

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (e1 flatElement) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes m ⊓ o.
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

// FlatInt generates a flat integer from the known integer value v.
func (elementFactory) FlatInt(v int) FlatIntElement {
	return FlatIntElement{
		element{flatIntLattice},
		v,
	}
}

// IValue retrieves the underlying value of the flat element with the given type.
func (e FlatIntElement) IValue() int {
	return e.value
}

// Value retrieves the underlying value of the flat element as an interface.
func (e FlatIntElement) Value() any {
	return e.value
}

// Is checks for equality with the value of `x`.
func (e FlatIntElement) Is(x any) bool {
	return e.value == x
}

// IsBot is false for non-⊥ elements.
func (e FlatIntElement) IsBot() bool {
	return false
}

// IsTop is false for non-⊤ elements.
func (e FlatIntElement) IsTop() bool {
	return false
}

// Flat safely converts to a flat element.
func (e FlatIntElement) Flat() FlatElement {
	return e
}

// FlatInt safely converts to a flat integer.
func (e FlatIntElement) FlatInt() FlatIntElement {
	return e
}

func (e FlatIntElement) String() string {
	return colorize.Element(strconv.Itoa(e.value))
}

// Height always returns 1 for known members of flat lattices.
func (e FlatIntElement) Height() int {
	return 1
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (e1 FlatIntElement) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes m ⊑ o.
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

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (e1 FlatIntElement) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes m ⊒ o.
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

// Eq computes m = o. Performs lattice dynamic type checking.
func (e1 FlatIntElement) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes m = o.
func (e1 FlatIntElement) eq(e2 Element) bool {
	return e1.leq(e2) && e1.geq(e2)
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (e1 FlatIntElement) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

// join computes m ⊔ o.
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

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (e1 FlatIntElement) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// meet computes m ⊓ o.
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
