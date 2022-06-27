package lattice

import (
	"Goat/analysis/defs"
)

//go:generate go run generate-map.go AnalysisIntraprocess defs.CtrLoc AnalysisState

var analysisIntraprocessLattice = &AnalysisIntraprocessLattice{mapLatticeBase{rng: analysisStateLattice}}

func (latticeFactory) AnalysisIntraprocess() *AnalysisIntraprocessLattice {
	return analysisIntraprocessLattice
}

func (a *AnalysisIntraprocessLattice) Bot() Element {
	el := element{a}
	return AnalysisIntraprocess{
		el,
		baseMap{el, defs.NewControllocationMap()},
	}
}

func (a *AnalysisIntraprocessLattice) String() string {
	return colorize.Lattice("ControlLoc") + " → " + a.rng.String()
}

func (elementFactory) AnalysisIntraprocess() AnalysisIntraprocess {
	return analysisIntraprocessLattice.Bot().AnalysisIntraprocess()
}
