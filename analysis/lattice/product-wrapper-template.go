//go:build ignore
// +build ignore

package lattice

import (
	"strings"

	"fmt"
	i "github.com/cs-au-dk/goat/utils/indenter"
)

type WrappedProductElement struct {
	element
	product Product
}

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

func (w WrappedProductElement) Height() int {
	return w.product.Height()
}

func (m WrappedProductElement) Eq(o Element) bool {
	checkLatticeMatch(m.lattice, o.Lattice(), "=")
	return m.eq(o)
}

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

func (m WrappedProductElement) Geq(o Element) bool {
	checkLatticeMatch(m.lattice, o.Lattice(), "⊒")
	return m.geq(o)
}

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

func (m WrappedProductElement) Leq(o Element) bool {
	checkLatticeMatch(m.lattice, o.Lattice(), "⊑")
	return m.leq(o)
}

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

func (m WrappedProductElement) Join(o Element) Element {
	checkLatticeMatch(m.Lattice(), o.Lattice(), "⊔")
	return m.join(o)
}

func (m WrappedProductElement) MonoJoin(o WrappedProductElement) WrappedProductElement {
	m.product = m.product.MonoJoin(o.product)
	return m
}

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

func (m WrappedProductElement) Meet(o Element) Element {
	checkLatticeMatch(m.lattice, o.Lattice(), "⊓")
	return m.meet(o)
}

func (m WrappedProductElement) MonoMeet(o WrappedProductElement) WrappedProductElement {
	m.product = m.product.MonoMeet(o.product)
	return m
}

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
