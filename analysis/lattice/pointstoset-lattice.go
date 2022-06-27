package lattice

import (
	loc "Goat/analysis/location"
	"Goat/utils/tree"
)

type PointsToLattice struct {
	lattice
}

var pointsToLattice *PointsToLattice = &PointsToLattice{}

func (m *PointsToLattice) Bot() Element {
	return PointsTo{
		element: element{lattice: pointsToLattice},
		mem:     tree.NewTree[loc.Location, struct{}](loc.LocationHasher{}),
	}
}

func (latticeFactory) PointsTo() *PointsToLattice {
	return pointsToLattice
}

/* Lattice boilerplate */
func (m *PointsToLattice) Eq(o Lattice) bool {
	return m == o
}

func (m *PointsToLattice) String() string {
	return colorize.Lattice("PointsTo")
}

func (m *PointsToLattice) Top() Element {
	panic(errUnsupportedOperation)
}
