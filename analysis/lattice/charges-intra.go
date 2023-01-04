package lattice

import (
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/tree"
)

//go:generate go run generate-map.go charges-intra

// chargeLattice is a singleton representation of the charges lattice.
// It maps control locations to sets of control locations.
// There is an edge x -> y for every y ∈ e(x).
// The meaning of the edges is overloaded - they are used both for returns and for
// specifying which deferred nodes are activated.
var chargesLattice = &ChargesLattice{mapLatticeBase{rng: MakeInfSetLatticeH[defs.CtrLoc]()}}

func (latticeFactory) ChargesIntra() *ChargesLattice {
	return chargesLattice
}

func (m *ChargesLattice) Bot() Element {
	el := element{lattice: m}
	return Charges{
		element: el,
		base:    tree.NewTree[defs.CtrLoc, InfSet[defs.CtrLoc]](utils.HashableHasher[defs.CtrLoc]()),
	}
}

/* Lattice boilerplate */
func (m *ChargesLattice) String() string {
	return colorize.Lattice("Charges")
}

// Charges produces a fresh charge element, with the single binding of control location
// `from` to all control locations in the list.
func (elementFactory) Charges(from defs.CtrLoc, locs ...defs.CtrLoc) Charges {
	ch := chargesLattice.Bot().Charges()
	for _, loc := range locs {
		ch = ch.Add(from, loc)
	}
	return ch
}

// Add control location `to` to set of charges bound to `from`.
func (ch Charges) Add(from, to defs.CtrLoc) Charges {
	prev, _ := ch.Get(from)
	return ch.Update(from, prev.Add(to))
}

// HasEdge checks whether `to` is charged for control location `from`.
func (ch Charges) HasEdge(from, to defs.CtrLoc) bool {
	mp, _ := ch.Get(from)
	return mp.Contains(to)
}

// Edges aggregates all charged control locations of `from`.
func (ch Charges) Edges(from defs.CtrLoc) (ret []defs.CtrLoc) {
	mp, _ := ch.Get(from)
	mp.ForEach(func(el defs.CtrLoc) {
		ret = append(ret, el)
	})
	return
}
