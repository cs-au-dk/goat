package lattice

import (
	loc "Goat/analysis/location"
)

//go:generate go run generate-product.go Cond Locker,Element,Element,Locker

type CondLattice struct {
	ProductLattice
}

var condLattice = &CondLattice{
	*latFact.Product(Drop(pointsToLattice)),
}

func (latticeFactory) Cond() *CondLattice {
	return condLattice
}

func (CondLattice) String() string {
	return colorize.Lattice("Cond")
}

func (l CondLattice) Top() Element {
	return Cond{
		element{condLattice},
		l.ProductLattice.Top().Product(),
	}
}

func (l CondLattice) Bot() Element {
	return Cond{
		element{condLattice},
		l.ProductLattice.Bot().Product(),
	}
}

func (l1 CondLattice) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	case *CondLattice:
		return true
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	}
	return false
}

func (elementFactory) Cond(locs ...loc.Location) Cond {
	return Cond{
		element{condLattice},
		elFact.Product(&condLattice.ProductLattice)(
			elFact.PointsTo(locs...),
		),
	}
}

func (c Cond) IsLockerKnown() bool {
	_, ok := c.product.Get(0).(*DroppedTop)
	return !ok
}

func (c Cond) AddLocker(loc loc.Location) Cond {
	if c.IsLockerKnown() {
		return c.UpdateLocker(c.KnownLockers().Add(loc))
	}
	return c
}

func (c Cond) HasLockers() bool {
	return !c.IsLockerKnown() || len(c.KnownLockers().NonNilEntries()) > 0
}

func (c Cond) KnownLockers() PointsTo {
	return c.Locker().PointsTo()
}
