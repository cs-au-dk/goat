package lattice

import (
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/tree"
)

//go:generate go run generate-map.go Charges defs.CtrLoc InfSet[defs.CtrLoc] tree

// An element e is a map from control locations to sets of control locations.
// There is an edge x -> y for every y ∈ e(x).
// The meaning of the edges is overloaded - they are used both for returns and for
// specifying which deferred nodes are activated.
var chargesLattice = &ChargesLattice{mapLatticeBase{rng: MakeInfSetLatticeH[defs.CtrLoc]()}}
//MakeInfiniteMapLatticeWFactory(twoElementLattice, "CtrLoc", utils.NewImmMap[defs.CtrLoc, Element]),

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

func (elementFactory) Charges(from defs.CtrLoc, locs ...defs.CtrLoc) Charges {
	ch := chargesLattice.Bot().Charges()
	for _, loc := range locs {
		ch = ch.Add(from, loc)
	}
	return ch
}

func (ch Charges) Add(from, to defs.CtrLoc) Charges {
	prev, _ := ch.Get(from)
	return ch.Update(from, prev.Add(to))
}

func (ch Charges) HasEdge(from, to defs.CtrLoc) bool {
	mp, _ := ch.Get(from)
	return mp.Contains(to)
}

func (ch Charges) Edges(from defs.CtrLoc) (ret []defs.CtrLoc) {
	mp, _ := ch.Get(from)
	mp.ForEach(func(el defs.CtrLoc) {
		ret = append(ret, el)
	})
	return
}
