package lattice

import (
	loc "github.com/cs-au-dk/goat/analysis/location"
)

//go:generate go run generate-product.go cond

// CondLattice represents the lattice of abstract Cond values. Its
// members are points-to sets encoding the allocation sites of potential
// lockers.
type CondLattice struct {
	ProductLattice
}

// condLattice is a singleton instantiation of the Cond lattice.
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

// IsLockerKnown checks whether the value of the associated lock is known.
// It verifies this by checking that the Locker field is not ⊤.
func (c Cond) IsLockerKnown() bool {
	_, ok := c.product.Get(0).(*DroppedTop)
	return !ok
}

// AddLocker adds a locker location site to the set of lockers of the abstract Cond value.
func (c Cond) AddLocker(loc loc.Location) Cond {
	if c.IsLockerKnown() {
		return c.UpdateLocker(c.KnownLockers().Add(loc))
	}
	return c
}

// HasLockers checks that the Cond has either the unknown locker, or at least
// one non-nil known locker.
func (c Cond) HasLockers() bool {
	return !c.IsLockerKnown() || len(c.KnownLockers().NonNilEntries()) > 0
}

// KnownLockers extracts the known lockers as a points-to set. Will panic if the lockers
// are not known.
func (c Cond) KnownLockers() PointsTo {
	return c.Locker().PointsTo()
}
