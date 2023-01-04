package absint

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	"github.com/cs-au-dk/goat/utils/graph"
	"github.com/cs-au-dk/goat/utils/hmap"
	"github.com/cs-au-dk/goat/utils/worklist"
)

type (
	// getSuccResult is the result of a successor computation, and
	// is a short-hand for a successor-abstract state pair.
	getSuccResult = struct {
		Successor
		State L.AnalysisState
	}

	// transfers maps successor hashes to successor results.
	// It is used by the abstract interpreter to expand or join
	// results in the superlocation graph.
	transfers map[uint32]getSuccResult
)

// succUpdate updates a set of successors with a new successor.
// Identical successors in the same set have their abstract states joined.
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

// SuperlocGraph is a graph of superlocations.
type SuperlocGraph struct {
	// entry is the superlocation acting as entry point
	entry *AbsConfiguration
	// Used to canonicalize abstract configurations such that configurations
	// with the same superlocation use the same AbsConfiguration object.
	canon *hmap.Map[defs.Superloc, *AbsConfiguration]
}

func (G SuperlocGraph) Size() int {
	//return len(G.graph)
	res := 0
	G.ForEach(func(_ *AbsConfiguration) {
		res++
	})
	return res
}

// PrettyPrint prints a textual representation of the superlocation graph
// to standard output.
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

// Entry retrieves the entry superlocation of a superlocation graph.
func (G SuperlocGraph) Entry() *AbsConfiguration {
	return G.entry
}

// GetOrSet retrieves the stateful abstract configuration from a superlocation, or otherwise
// inserts it into the graph if was previously absent.
func (G SuperlocGraph) GetOrSet(s *AbsConfiguration) *AbsConfiguration {
	if _, found := G.canon.GetOk(s.Superloc); !found {
		G.canon.Set(s.Superloc, s)
	}
	return G.canon.Get(s.Superloc)
}

// Get retrieves an abstract configuration corresponding to the given superlocation
// from the superlocation graph.
func (G SuperlocGraph) Get(sl defs.Superloc) *AbsConfiguration {
	return G.canon.Get(sl)
}

// ForEach executes the given procedure for each superlocation in the graph
// in breadth-first order starting at the entry.
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

// Terminals returns all terminal configurations i.e., without any successors.
func (G SuperlocGraph) Terminals() map[*AbsConfiguration]struct{} {
	terminals := make(map[*AbsConfiguration]struct{})

	G.ForEach(func(s *AbsConfiguration) {
		if len(s.Successors) == 0 {
			terminals[s] = struct{}{}
		}
	})

	return terminals
}

// ToGraph converts a superloc graph to a graph of abstract configurations.
func (G SuperlocGraph) ToGraph() graph.Graph[*AbsConfiguration] {
	return graph.OfHashable(func(conf *AbsConfiguration) (res []*AbsConfiguration) {
		for _, succ := range conf.Successors {
			if next := succ.Configuration(); !next.IsPanicked() {
				res = append(res, next)
			}
		}
		return
	})
}
