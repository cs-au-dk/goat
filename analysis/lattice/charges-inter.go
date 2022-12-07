package lattice

import (
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/utils"
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
		baseMap[defs.Goro]{el, utils.NewImmMap[defs.Goro, Element]()},
	}
}

/* Lattice boilerplate */
func (m *ThreadChargesLattice) String() string {
	return colorize.Lattice("ThreadCharges")
}

func (elementFactory) ThreadCharges() ThreadCharges {
	return threadChargesLattice.Bot().ThreadCharges()
}
