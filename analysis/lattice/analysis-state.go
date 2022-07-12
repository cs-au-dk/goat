package lattice

import (
	"Goat/analysis/defs"
	loc "Goat/analysis/location"
)

//go:generate go run generate-product.go AnalysisState Stack,AnalysisStateStack,AnalysisStateStack,Stack Heap,Memory,Memory,Heap ThreadCharges,ThreadCharges,ThreadCharges,Charges

type AnalysisStateLattice struct {
	ProductLattice
}

func (latticeFactory) AnalysisState() *AnalysisStateLattice {
	return analysisStateLattice
}

var analysisStateLattice *AnalysisStateLattice = &AnalysisStateLattice{
	*latFact.Product(
		// Local memory - binds SSA register names
		analysisStateStackLattice,
		// Heap memory - binds arbitrary heap locations
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

func (elementFactory) AnalysisState(stack AnalysisStateStack, heap Memory, ch ThreadCharges) AnalysisState {
	return AnalysisState{
		element: element{analysisStateLattice},
		product: elFact.Product(analysisStateLattice.Product())(
			stack,
			heap,
			ch,
		),
	}
}

func (s AnalysisState) Get(l loc.Location) (AbstractValue, bool) {
	switch l := l.(type) {
	case loc.LocalLocation:
		g := l.Goro.(defs.Goro)
		return s.Stack().GetUnsafe(g).Get(l)
	}
	return MemOps(s.Heap()).Get(l)
}
func (s AnalysisState) Update(l loc.Location, v AbstractValue) AnalysisState {
	switch l := l.(type) {
	case loc.LocalLocation:
		g := l.Goro.(defs.Goro)
		gStack, ok := s.Stack().Get(g)
		if ok {
			gStack = gStack.Update(l, v)
		} else {
			gStack = Consts().FreshMemory().Update(l, v)
		}
		return s.UpdateStack(s.Stack().Update(g, gStack))
	}

	heap := MemOps(s.Heap()).Update(l, v).Memory()
	return s.UpdateHeap(heap)
}

func (s AnalysisState) GetUnsafe(l loc.Location) (v AbstractValue) {
	switch l := l.(type) {
	case loc.LocalLocation:
		return s.Stack().GetUnsafe(l.Goro.(defs.Goro)).GetUnsafe(l)
	}
	return MemOps(s.Heap()).GetUnsafe(l)
}

func (s AnalysisState) UpdateMemory(stack AnalysisStateStack, heap Memory) AnalysisState {
	return s.UpdateStack(stack).UpdateHeap(heap)
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

type Charge = struct {
	defs.CtrLoc
	Memory
}

func (s AnalysisState) AddCharges(tid defs.Goro, charges ...Charge) AnalysisState {
	return s.UpdateThreadCharges(
		s.ThreadCharges().WeakUpdate(tid, elFact.Charges(charges...)),
	)
}

func (s AnalysisState) HeapAlloc(loc loc.AllocationSiteLocation, initVal AbstractValue) AnalysisState {
	hops := MemOps(s.Heap())
	hops.HeapAlloc(loc, initVal)
	return s.UpdateHeap(hops.Memory())
}

func (s AnalysisState) Allocate(loc loc.AllocationSiteLocation, initVal AbstractValue, forceMultiAlloc bool) AnalysisState {
	return s.UpdateHeap(s.Heap().Allocate(loc, initVal, forceMultiAlloc))
}

func (s AnalysisState) UpdateThreadStack(g defs.Goro, stack Memory) AnalysisState {
	return s.UpdateStack(s.Stack().Update(g, stack))
}

// func (a AnalysisState) ProjectMemory() string {
// 	stackBuf := make([]func() string, 0, a.Stack().Size())
// 	heapBuf := make([]func() string, 0, a.Heap().Size())

// 	a.Stack().ForEach(func(al loc.AddressableLocation, av AbstractValue) {
// 		stackBuf = append(stackBuf, func() string {
// 			return fmt.Sprintf("%s ↦ %s", al, av)
// 		})
// 	})
// 	a.Heap().ForEach(func(al loc.AddressableLocation, av AbstractValue) {
// 		heapBuf = append(heapBuf, func() string {
// 			return fmt.Sprintf("%s ↦ %s", al, av)
// 		})
// 	})

// 	return i.Indenter().Start("").NestThunked(
// 		func() string {
// 			return i.Indenter().Start(colorize.Lattice("Stack") + ": {").NestThunked(stackBuf...).End("}\n")
// 		},
// 		func() string {
// 			return i.Indenter().Start(colorize.Lattice("End") + ": {").NestThunked(heapBuf...).End("}")
// 		}).End("")
// }

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
