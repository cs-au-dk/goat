package lattice

import (
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/utils"
)

//go:generate go run generate-map.go analysis-intraprocess

// analysisIntraprocessLattice is a singleton instantiation
// of the intra-processual analysis result lattice.
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

// AnalysisIntraprocess bottom element.
func (elementFactory) AnalysisIntraprocess() AnalysisIntraprocess {
	return analysisIntraprocessLattice.Bot().AnalysisIntraprocess()
}
