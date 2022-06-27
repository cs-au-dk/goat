package lattice

import (
	"Goat/analysis/defs"
)

//go:generate go run generate-map.go Charges defs.CtrLoc twoElementLatticeElement

var chargesLattice = &ChargesLattice{mapLatticeBase{rng: twoElementLattice}}

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

func (elementFactory) Charges(locs ...defs.CtrLoc) Charges {
	ch := chargesLattice.Bot().Charges()
	for _, loc := range locs {
		ch = ch.Add(loc)
	}
	return ch
}

func (ch Charges) Add(node defs.CtrLoc) Charges {
	return ch.Update(node, true)
}
