//go:build ignore
// +build ignore

package lattice

import (
	"strings"

	"fmt"
	i "github.com/cs-au-dk/goat/utils/indenter"
)

// WrappedProductElement WrappedProductDescription
type WrappedProductElement struct {
	element
	product Product
}

// WrappedProductElement safely converts the InformalProductElement.
func (m WrappedProductElement) WrappedProductElement() WrappedProductElement {
	return m
}

func (w WrappedProductElement) String() string {
	fields := []struct {
		name  string
		value Element
	}{
		STRING_ENTRIES,
	}

	strs := make([]func() string, 0, len(fields))

	for _, field := range fields {
		if !field.value.eq(field.value.Lattice().Bot()) {
			strs = append(strs, (func(name string, value Element) func() string {
				return func() string {
					return colorize.Field(name) + ": " + value.String()
				}
			})(field.name, field.value))
		}
	}

	if len(strs) == 0 {
		return colorize.Element("⊥")
	}

	return i.Indenter().Start("{").NestThunkedSep(",", strs...).End("}")
}

// Height computes the height of the InformalProductElement in its corresponding lattice.
func (w WrappedProductElement) Height() int {
	return w.product.Height()
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (m WrappedProductElement) Eq(o Element) bool {
	checkLatticeMatch(m.lattice, o.Lattice(), "=")
	return m.eq(o)
}

// eq computes m = o.
func (m WrappedProductElement) eq(o Element) bool {
	switch o := o.(type) {
	case WrappedProductElement:
		return m.product.eq(o.product)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (m WrappedProductElement) Geq(o Element) bool {
	checkLatticeMatch(m.lattice, o.Lattice(), "⊒")
	return m.geq(o)
}

// geq computes m ⊒ o.
func (m WrappedProductElement) geq(o Element) bool {
	switch o := o.(type) {
	case WrappedProductElement:
		return m.product.geq(o.product)
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (m WrappedProductElement) Leq(o Element) bool {
	checkLatticeMatch(m.lattice, o.Lattice(), "⊑")
	return m.leq(o)
}

// leq computes m ⊑ o.
func (m WrappedProductElement) leq(o Element) bool {
	switch o := o.(type) {
	case WrappedProductElement:
		return m.product.leq(o.product)
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (m WrappedProductElement) Join(o Element) Element {
	checkLatticeMatch(m.Lattice(), o.Lattice(), "⊔")
	return m.join(o)
}

// MonoJoin is the monomorphic variant of m ⊔ o for InformalProductElement.
func (m WrappedProductElement) MonoJoin(o WrappedProductElement) WrappedProductElement {
	m.product = m.product.MonoJoin(o.product)
	return m
}

// join computes m ⊔ o.
func (m WrappedProductElement) join(o Element) Element {
	switch o := o.(type) {
	case WrappedProductElement:
		return m.MonoJoin(o)
	case *LiftedBot:
		return m
	case *DroppedTop:
		return o
	default:
		panic(errInternal)
	}
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (m WrappedProductElement) Meet(o Element) Element {
	checkLatticeMatch(m.lattice, o.Lattice(), "⊓")
	return m.meet(o)
}

// MonoMeet is the monomorphic variant of m ⊓ o for members of the WrappedProductLattice.
func (m WrappedProductElement) MonoMeet(o WrappedProductElement) WrappedProductElement {
	m.product = m.product.MonoMeet(o.product)
	return m
}

// meet computes m ⊓ o.
func (m WrappedProductElement) meet(o Element) Element {
	switch o := o.(type) {
	case WrappedProductElement:
		return m.MonoMeet(o)
	case *LiftedBot:
		return o
	case *DroppedTop:
		return m
	default:
		panic(errInternal)
	}
}
