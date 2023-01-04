package lattice

import (
	"fmt"

	i "github.com/cs-au-dk/goat/utils/indenter"
)

// Product is a member of a product lattice.
type Product struct {
	element
	// The arity of the products we use is small (the list of elements is
	// short). In this case copying slices is cheaper than modifying elements
	// in immutable lists that are made for fast updates in really long lists.
	// (The reason pointer-to slice is used is to retain comparability of
	// elements (used in various places, such as value.go)).
	prod *[]Element
}

func (p Product) Product() Product {
	return p
}

// newProduct creates a fresh element for the given product lattice.
func newProduct(lat Lattice) Product {
	lat2 := lat.Product()
	sl := make([]Element, len(lat2.product))
	for i, lat := range lat2.product {
		sl[i] = lat.Bot()
	}
	return Product{element{lat2}, &sl}
}

// Product creates a product factory for the given lattice.
func (elementFactory) Product(lat Lattice) func(members ...Element) Product {
	switch lat := lat.(type) {
	case *ProductLattice:
		return func(members ...Element) Product {
			el := newProduct(lat)
			for i, e2 := range members {
				checkLatticeMatchThunked(lat.product[i], e2.Lattice(), func() string {
					return fmt.Sprintf("(%s)(%d) := %s", el, i, e2)
				})
				(*el.prod)[i] = e2
			}
			return el
		}
	case *Dropped:
		return elFact.Product(lat.Lattice)
	case *Lifted:
		return elFact.Product(lat.Lattice)
	default:
		panic("Attempted creating product with a non-product lattice")
	}
}

func (e Product) String() string {
	strs := []fmt.Stringer{}
	for _, e := range *e.prod {
		strs = append(strs, e)
	}
	return i.Indenter().Start("(").NestSep(",", strs...).End(")")
}

func (p Product) Height() (h int) {
	for index, e := range *p.prod {
		elat := p.Lattice().Product().Get(index)
		switch e := e.(type) {
		case *LiftedBot:
			h += elat.Preheight() - (e.Index() + 1)
		default:
			h += elat.Preheight() + e.Height()
		}
	}

	return
}

// forall checks that all members of the product satisfy a given property.
func (p Product) forall(f func(int, Element) bool) bool {
	for i, e := range *p.prod {
		if !f(i, e) {
			return false
		}
	}
	return true
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (e1 Product) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

// eq computes m = o.
func (p Product) eq(oe Element) bool {
	o, ok := oe.(Product)
	if !ok {
		return false
	}

	return p.forall(func(i int, e Element) bool {
		return e.eq(o.Get(i))
	})
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (e1 Product) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

// geq computes m ⊒ o.
func (e1 Product) geq(e2 Element) bool {
	switch e2 := e2.(type) {
	case Product:
		return e1.forall(func(i int, e Element) bool {
			return e.geq(e2.Get(i))
		})
	case *LiftedBot:
		return true
	case *DroppedTop:
		return false
	default:
		panic(errInternal)
	}
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (e1 Product) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

// leq computes m ⊑ o.
func (e1 Product) leq(e2 Element) bool {
	switch e2 := e2.(type) {
	case Product:
		return e1.forall(func(i int, e Element) bool {
			return e.leq(e2.Get(i))
		})
	case *LiftedBot:
		return false
	case *DroppedTop:
		return true
	default:
		panic(errInternal)
	}
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (e1 Product) Join(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊔")
	return e1.join(e2)
}

// MonoJoin is a monomorphic variant of m ⊔ o for products.
func (e1 Product) MonoJoin(e2 Product) Product {
	// Improves performance a lot.
	if e1.prod == e2.prod {
		return e1
	}

	sl := make([]Element, len(*e1.prod))
	for i, e := range *e1.prod {
		sl[i] = e.join(e2.Get(i))
	}

	e1.prod = &sl
	return e1
}

// join computes m ⊔ o.
func (e1 Product) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case Product:
		return e1.MonoJoin(e2)
	case *LiftedBot:
		return e1
	case *DroppedTop:
		return e2
	default:
		panic(errInternal)
	}
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (e1 Product) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

// MonoMeet is a monomorphic variant of m ⊓ o for inter-processual analysis result.
func (e1 Product) MonoMeet(e2 Product) Product {
	// Improves performance a lot.
	if e1.prod == e2.prod {
		return e1
	}

	sl := make([]Element, len(*e1.prod))
	for i, e := range *e1.prod {
		sl[i] = e.meet(e2.Get(i))
	}

	e1.prod = &sl
	return e1
}

// meet computes m ⊓ o.
func (e1 Product) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case Product:
		return e1.MonoMeet(e2)
	case *LiftedBot:
		return e2
	case *DroppedTop:
		return e1
	default:
		panic(errInternal)
	}
}

// Update retrieves a product where its `i`-th member has been updated
// to the given value `e2`. Performs lattice dynamic type checking.
func (e1 Product) Update(i int, e2 Element) Product {
	prodLat := e1.lattice.Product()
	if i < 0 || len(prodLat.product) <= i {
		panic(fmt.Sprintf("Invalid index %d for product lattice:\n%s", i, prodLat))
	}
	checkLatticeMatchThunked(prodLat.product[i], e2.Lattice(), func() string {
		return fmt.Sprintf("(%s)(%d) := %s", e1, i, e2)
	})

	return e1.update(i, e2)
}

// update retrieves a product where its `i`-th member has been updated
// to the given value `e2`.
func (e1 Product) update(i int, e2 Element) Product {
	sl := make([]Element, len(*e1.prod))
	copy(sl, *e1.prod)
	sl[i] = e2
	e1.prod = &sl
	return e1
}

// Get retrieves the `i`-th element of the product.
func (e Product) Get(i int) Element {
	return (*e.prod)[i]
}
