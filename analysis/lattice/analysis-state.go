package lattice

import (
	"github.com/cs-au-dk/goat/analysis/defs"
)

//go:generate go run generate-product.go analysis-state

// AnalysisStateLattice represents the lattice of analysis results at a single superlocation.
type AnalysisStateLattice struct {
	ProductLattice
}

func (latticeFactory) AnalysisState() *AnalysisStateLattice {
	return analysisStateLattice
}

// analysisStateLattice is a singleton instantiation of the lattice of
// analysis results at a single superlocation.
var analysisStateLattice *AnalysisStateLattice = &AnalysisStateLattice{
	*latFact.Product(
		memoryLattice,
		threadChargesLattice,
	),
}

func (c *AnalysisStateLattice) Top() Element {
	panic(errUnsupportedOperation)
}

func (c *AnalysisStateLattice) Bot() Element {
	return AnalysisState{
		element: element{lattice: c},
		product: c.ProductLattice.Bot().Product(),
	}
}

func init() {
	_checkAnalysisState(analysisStateLattice.Bot().(AnalysisState))
}

// AnalysisState produces a new analysis state, given the starting abstract memory and
// bindings of thread charges.
func (elementFactory) AnalysisState(mem Memory, ch ThreadCharges) AnalysisState {
	return AnalysisState{
		element: element{analysisStateLattice},
		product: elFact.Product(analysisStateLattice.Product())(
			mem,
			ch,
		),
	}
}

func (l1 *AnalysisStateLattice) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	case *AnalysisStateLattice:
		return true
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (*AnalysisStateLattice) String() string {
	return colorize.Lattice("AnalysisState")
}

func (c *AnalysisStateLattice) Product() *ProductLattice {
	return c.ProductLattice.Product()
}

func (c *AnalysisStateLattice) AnalysisState() *AnalysisStateLattice {
	return c
}

// AddCharge charges control location `to` for `from` at abstract thread `tid`.
func (s AnalysisState) AddCharge(tid defs.Goro, from defs.CtrLoc, to defs.CtrLoc) AnalysisState {
	return s.UpdateThreadCharges(
		s.ThreadCharges().WeakUpdate(tid, elFact.Charges(from, to)),
	)
}
