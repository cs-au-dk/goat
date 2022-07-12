package lattice

import (
	"Goat/analysis/defs"
)

//go:generate go run generate-map.go Charges defs.CtrLoc Memory

var chargesLattice = &ChargesLattice{mapLatticeBase{rng: memoryLattice}}

func (latticeFactory) ChargesIntra() *ChargesLattice {
	return chargesLattice
}

func (m *ChargesLattice) Bot() Element {
	el := element{lattice: m}
	return Charges{
		element: el,
		base:    baseMap{el, defs.NewControllocationMap()},
	}
}

/* Lattice boilerplate */
func (m *ChargesLattice) String() string {
	return colorize.Lattice("Charges")
}

func (elementFactory) Charges(chgs ...Charge) Charges {
	ch := chargesLattice.Bot().Charges()
	for _, cg := range chgs {
		ch = ch.Update(cg.CtrLoc, cg.Memory)
	}
	return ch
}
