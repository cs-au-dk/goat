package absint

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/cs-au-dk/goat/utils/pq"

	L "github.com/cs-au-dk/goat/analysis/lattice"
)

// Harness for performing fully static analysis.
// Accepts entry abstract configuration node as input and generates an analysis result.
func StaticAnalysis(C AnalysisCtxt) (SuperlocGraph, L.Analysis) {
	// Channel for catching SIGUSR1 signals
	sigCh := make(chan os.Signal, 10)
	signal.Notify(sigCh, syscall.SIGUSR1)
	defer signal.Stop(sigCh)

	// Create initial configuration
	s0 := C.InitConf
	initState := C.InitState

	// Create initial analysis lattice
	analysis := Elements().Analysis()
	// Create an initial configuration graph
	G := Create().SuperlocGraph(s0)

	// Instantiate worklist based static analysis
	analysis = analysis.Update(s0.Superlocation(), initState)

	// Create a prioritized worklist. Initially every configuration has the
	// same priority. The worklist also keeps track of which superlocations are
	// in the queue to avoid duplicate work.
	priorities := map[*AbsConfiguration]int{}
	getPrio := func(a *AbsConfiguration) int {
		if prio, found := priorities[a]; found {
			return prio
		}
		return -1 // (Process first, to explore new edges)
	}
	worklist := pq.Empty(func(a, b *AbsConfiguration) bool {
		return getPrio(a) < getPrio(b)
	})
	worklist.Add(s0)

	// Configurations are reprioritized at an exponentially decreasing rate
	reprioritizeAt := 50
FIXPOINT:
	for steps := 0; !worklist.IsEmpty(); steps++ {
		if C.Metrics.Enabled() {
			select {
			case <-C.Metrics.skipped:
				break FIXPOINT
			default:
			}
		}

		select {
		case <-sigCh:
			// Received SIGUSR1 interrupt
			G.Visualize(nil)
		default:
		}

		if steps == reprioritizeAt {
			// Reprioritize configurations
			scc := G.ToGraph().SCC([]*AbsConfiguration{G.Entry()})
			priorities = map[*AbsConfiguration]int{}
			for compIdx, component := range scc.Components {
				for _, conf := range component {
					// Prioritize in topological order
					priorities[conf] = len(scc.Components) - compIdx - 1
				}
			}

			// Remove elements from the worklist that are not reachable
			// anymore. This sounds unlikely but happens sometimes because
			// previously discovered transitions disappear after re-analysis.
			// This indicates that the analysis is not monotone, which can for
			// example be due to GH issue #12.
			// When this happens the unreachable superlocations will always
			// have priority -1, which is undesirable!
			elements := []*AbsConfiguration{}
			for !worklist.IsEmpty() {
				c := worklist.GetNext()
				if _, found := priorities[c]; found {
					elements = append(elements, c)
				}
			}

			for _, c := range elements {
				worklist.Add(c)
			}

			reprioritizeAt *= 2
		}

		s := worklist.GetNext()
		C.LogSuperlocation(s.superloc)
		state := analysis.GetUnsafe(s.Superlocation())

		// Clear successor map to prevent duplicate edges.
		s.Successors = map[uint32]Successor{}
		succs := s.GetTransitions(C, state)
		for _, succ := range succs {
			s1 := G.GetOrSet(succ.Configuration())
			s1Loc := s1.Superlocation()
			// Add found successor to successor map, if not already present, and record
			// the added transition to the "state-less" successor map
			s.AddSuccessor(succ.DeriveConf(s1))

			// prevState becomes bot if not found
			prevState := analysis.GetOrBot(s1Loc)

			updState := succ.State

			if !s1.IsPanicked() {
				// If the memory was updated as a result of the LUB operation we put s1 in the worklist.
				if lub := updState.MonoJoin(prevState); !lub.Eq(prevState) {
					analysis = analysis.Update(s1Loc, lub)

					worklist.Add(s1)
				}
			} else {
				// Skip processing of panicked configurations
				analysis = analysis.Update(s1Loc, updState)
			}
		}
	}

	return G, analysis
}
