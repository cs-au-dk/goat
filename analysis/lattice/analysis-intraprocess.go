package lattice

import (
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/utils"
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
		baseMap[defs.CtrLoc]{el, utils.NewImmMap[defs.CtrLoc, Element]()},
	}
}

func (a *AnalysisIntraprocessLattice) String() string {
	return colorize.Lattice("ControlLoc") + " → " + a.rng.String()
}

func (elementFactory) AnalysisIntraprocess() AnalysisIntraprocess {
	return analysisIntraprocessLattice.Bot().AnalysisIntraprocess()
}
