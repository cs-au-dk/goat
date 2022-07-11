package lattice

import (
	"Goat/analysis/defs"
	loc "Goat/analysis/location"
)

//go:generate go run generate-map.go AnalysisStateStack defs.Goro Memory

var analysisStateStackLattice = &AnalysisStateStackLattice{mapLatticeBase{rng: memoryLattice}}

func (latticeFactory) AnalysisStateStack() *AnalysisStateStackLattice {
	return analysisStateStackLattice
}

func (a *AnalysisStateStackLattice) Bot() Element {
	el := element{a}
	return AnalysisStateStack{
		el,
		baseMap{el, defs.NewGoroutineMap()},
	}
}

func (*AnalysisStateStackLattice) String() string {
	return colorize.Lattice("Stacks:")
}

func (s AnalysisStateStack) UpdateLoc(l loc.LocalLocation, v AbstractValue) AnalysisStateStack {
	g := l.Goro.(defs.Goro)
	gStack := s.GetUnsafe(g)
	return s.Update(g, gStack.Update(l, v))
}

func (s AnalysisStateStack) GetLoc(l loc.LocalLocation) (AbstractValue, bool) {
	g := l.Goro.(defs.Goro)
	gStack := s.GetUnsafe(g)
	return gStack.Get(l)
}

func (s AnalysisStateStack) GetLocUnsafe(l loc.LocalLocation) AbstractValue {
	g := l.Goro.(defs.Goro)
	gStack := s.GetUnsafe(g)
	return gStack.GetUnsafe(l)
}
