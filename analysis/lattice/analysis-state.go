package lattice

import (
	"github.com/cs-au-dk/goat/analysis/defs"
)

//go:generate go run generate-product.go AnalysisState Memory,Memory,Memory,Memory ThreadCharges,ThreadCharges,ThreadCharges,Charges

type AnalysisStateLattice struct {
	ProductLattice
}

func (latticeFactory) AnalysisState() *AnalysisStateLattice {
	return analysisStateLattice
}

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

func (s AnalysisState) AddCharges(tid defs.Goro, locs ...defs.CtrLoc) AnalysisState {
	return s.UpdateThreadCharges(
		s.ThreadCharges().WeakUpdate(tid, elFact.Charges(locs...)),
	)
}

// func (a AnalysisState) ChanMemory() string {
// 	return a.Memory().Filter(func(_ loc.AddressableLocation, av AbstractValue) bool {
// 		switch {
// 		case av.IsChan():
// 			return true
// 		case av.IsPointer():
// 			for _, l := range av.PointerValue().NonNilEntries() {
// 				if l2, ok := l.(loc.AddressableLocation); ok {
// 					if av, ok := a.Memory().Get(l2); ok && av.IsChan() {
// 						return true
// 					}
// 				}
// 			}
// 		}
// 		return false
// 	}).String()
// }
