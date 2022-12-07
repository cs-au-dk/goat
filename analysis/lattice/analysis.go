package lattice

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/defs"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/utils"
	i "github.com/cs-au-dk/goat/utils/indenter"
)

//go:generate go run generate-map.go Analysis defs.Superloc AnalysisState

var analysisLattice = &AnalysisLattice{mapLatticeBase{rng: analysisStateLattice}}

func (latticeFactory) Analysis() *AnalysisLattice {
	return analysisLattice
}

func (a *AnalysisLattice) Bot() Element {
	el := element{a}
	return Analysis{
		el,
		baseMap[defs.Superloc]{el, utils.NewImmMap[defs.Superloc, Element]()},
	}
}

func (a *AnalysisLattice) String() string {
	return colorize.Lattice("Superloc") + " → " + a.rng.String()
}

func (elementFactory) Analysis() Analysis {
	return analysisLattice.Bot().Analysis()
}

func (a Analysis) GetOrBot(sl defs.Superloc) AnalysisState {
	return a.GetOrDefault(sl, analysisStateLattice.Bot().AnalysisState())
}

func (a Analysis) ProjectMemory() string {
	buf := make([]func() string, 0, a.Size())
	a.ForEach(func(s defs.Superloc, as AnalysisState) {
		buf = append(buf, func() string {
			return fmt.Sprintf("%s ↦ %s", s, as.Memory())
		})
	})

	return i.Indenter().Start(a.Lattice().String() + ": {").NestThunked(buf...).End("}")
}

func (a Analysis) ChanMemory() string {
	buf := make([]func() string, 0, a.Size())
	a.ForEach(func(s defs.Superloc, as AnalysisState) {
		buf = append(buf, func() string {
			return fmt.Sprintf("%s ↦ %s", s, as.Memory().Filter(func(_ loc.AddressableLocation, av AbstractValue) bool {
				switch {
				case av.IsChan():
					return true
				case av.IsPointer():
					for _, l := range av.PointerValue().NonNilEntries() {
						if l2, ok := l.(loc.AddressableLocation); ok {
							if av, ok := as.Memory().Get(l2); ok && av.IsChan() {
								return true
							}
						}
					}
				}
				return false
			}))
		})
	})

	return i.Indenter().Start(a.Lattice().String() + ": {").NestThunked(buf...).End("}")
}
