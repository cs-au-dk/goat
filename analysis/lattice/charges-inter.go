package lattice

import (
	"Goat/analysis/defs"
)

//go:generate go run generate-map.go ThreadCharges defs.Goro Charges

var threadChargesLattice = &ThreadChargesLattice{mapLatticeBase{rng: chargesLattice}}

func (latticeFactory) ThreadCharges() *ThreadChargesLattice {
	return threadChargesLattice
}

func (m *ThreadChargesLattice) Bot() Element {
	el := element{m}
	return ThreadCharges{
		el,
		baseMap{el, defs.NewGoroutineMap()},
	}
}

/* Lattice boilerplate */
func (m *ThreadChargesLattice) String() string {
	return colorize.Lattice("ThreadCharges")
}

func (elementFactory) ThreadCharges() ThreadCharges {
	return threadChargesLattice.Bot().ThreadCharges()
}
