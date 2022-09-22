package lattice

import (
	"fmt"

	i "github.com/cs-au-dk/goat/utils/indenter"
)

type ProductLattice struct {
	lattice
	top *Product
	bot *Product

	product []Lattice
}

func (latticeFactory) Product(lats ...Lattice) *ProductLattice {
	p := new(ProductLattice)
	p.product = lats

	return p
}

func (p *ProductLattice) Lattices() []Lattice {
	return p.product
}

func (p *ProductLattice) Product() *ProductLattice {
	return p
}

func (p *ProductLattice) Size() int {
	return len(p.product)
}

func (l1 *ProductLattice) Eq(l2 Lattice) bool {
	// First try to get away with referential equality
	if l1 == l2 {
		return true
	}
	switch l2 := l2.(type) {
	case *ProductLattice:
		if l1.Size() != l2.Size() {
			return false
		}
		for i := 0; i < l1.Size(); i++ {
			if !l1.product[i].Eq(l2.product[i]) {
				return false
			}
		}
		return true
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (p *ProductLattice) Top() Element {
	if p.top == nil {
		p.top = new(Product)
		*p.top = newProduct(p)
		for i, lat := range p.product {
			p.top.prod = p.top.prod.set(i, lat.Top())
		}
	}
	return *p.top
}

func (p *ProductLattice) Bot() Element {
	if p.bot == nil {
		p.bot = new(Product)
		*p.bot = newProduct(p)
	}

	return *p.bot
}

func (p *ProductLattice) String() string {
	strs := []fmt.Stringer{}
	for _, lat := range p.product {
		strs = append(strs, lat)
	}

	return i.Indenter().Start("(").NestSep(
		" ×",
		strs...,
	).End(")")
}

/* TODO: If this is needed we can add some extra code in Get/Update to handle
lazy updates */
func (p *ProductLattice) Extend(l Lattice) {
	p.product = append(p.product, l)
}

func (p *ProductLattice) Get(i int) Lattice {
	return p.product[i]
}
