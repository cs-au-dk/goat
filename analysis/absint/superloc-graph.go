package absint

import (
	L "Goat/analysis/lattice"
	"Goat/utils/graph"
	"Goat/utils/hmap"
	"Goat/utils/worklist"
	"fmt"
)

type transfers map[uint32]getSuccResult

func (S transfers) succUpdate(
	succ Successor,
	state L.AnalysisState,
) {
	sHash := succ.Configuration().Hash()
	if old, exists := S[sHash]; !exists {
		// Instantiate successor if no previous result was found.
		S[sHash] = getSuccResult{succ, state}
	} else {
		// Join with previous result if one already exists.
		S[sHash] = getSuccResult{
			succ,
			old.State.MonoJoin(state),
		}
	}
}

func (S transfers) String() (str string) {
	for _, succ := range S {
		str += succ.String() + "\n"
	}
	return
}

type SuperlocGraph struct {
	entry *AbsConfiguration
	// Used to canonicalize abstract configurations such that configurations
	// with the same superlocation use the same AbsConfiguration object.
	canon *hmap.Map[*AbsConfiguration, *AbsConfiguration]
}

func (G SuperlocGraph) Size() int {
	//return len(G.graph)
	res := 0
	G.ForEach(func(_ *AbsConfiguration) {
		res++
	})
	return res
}

func (G SuperlocGraph) PrettyPrint() {
	G.ForEach(func(s *AbsConfiguration) {
		fmt.Println("--------------")
		fmt.Println("Superlocation:")
		s.PrettyPrint()
		fmt.Println("Successors:")
		for _, conf := range s.Successors {
			conf.PrettyPrint()
			fmt.Println()
		}
	})
}

func (G SuperlocGraph) String() (str string) {
	G.ForEach(func(s *AbsConfiguration) {
		str += "--------------\n"
		str += "Superlocation:\n"
		str += s.String()
		str += "\nSuccessors:\n"
		for _, succ := range s.Successors {
			conf := succ.Configuration()
			str += conf.String() + "\n"
		}
	})
	return
}

func (G SuperlocGraph) Entry() *AbsConfiguration {
	return G.entry
}

func (G SuperlocGraph) Get(s *AbsConfiguration) *AbsConfiguration {
	if _, found := G.canon.GetOk(s); !found {
		G.canon.Set(s, s)
	}
	return G.canon.Get(s)
}

func (G SuperlocGraph) ForEach(do func(*AbsConfiguration)) {
	visited := hmap.NewMap[struct{}](achasher)
	visited.Set(G.Entry(), struct{}{})

	W := worklist.Empty[*AbsConfiguration]()
	W.Add(G.Entry())

	for !W.IsEmpty() {
		s := W.GetNext()

		do(s)
		for _, succ := range s.Successors {
			s := succ.Configuration()
			if _, found := visited.GetOk(s); !found {
				visited.Set(s, struct{}{})
				W.Add(s)
			}
		}
	}
}

// Returns all terminal configurations
func (G SuperlocGraph) Terminals() map[*AbsConfiguration]struct{} {
	terminals := make(map[*AbsConfiguration]struct{})

	G.ForEach(func(s *AbsConfiguration) {
		if len(s.Successors) == 0 {
			terminals[s] = struct{}{}
		}
	})

	return terminals
}

func (G SuperlocGraph) ToGraph() graph.Graph[*AbsConfiguration] {
	return graph.OfHashable(func(conf *AbsConfiguration) (res []*AbsConfiguration) {
		for _, succ := range conf.GetSuccessorMap() {
			if next := succ.Configuration(); !next.IsPanicked() {
				res = append(res, next)
			}
		}
		return
	})
}
