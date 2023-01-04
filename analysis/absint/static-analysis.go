package absint

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/cs-au-dk/goat/utils/pq"

	L "github.com/cs-au-dk/goat/analysis/lattice"
)

// StaticAnalysis performs a full static analysis, based on the given analysis context.
// The generated result is a member of the analysis lattice, and the superlocation graph
// constructed from the elements not bound to ⊥ in the result domain.
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
	analysis = analysis.Update(s0.Superloc, initState)

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

	// Construct worklist from priority queue.
	worklist := pq.Empty(func(a, b *AbsConfiguration) bool {
		return getPrio(a) < getPrio(b)
	})
	worklist.Add(s0)

	// Configurations are reprioritized at an exponentially decreasing rate
	reprioritizeAt := 50

	// Compute analysis fixed-point.
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

		// Retrieve next superlocation and the associated abstract state
		// from the worklist.
		s := worklist.GetNext()
		C.LogSuperlocation(s.Superloc)
		state := analysis.GetUnsafe(s.Superloc)

		// Reset successor map to prevent duplicate edges.
		s.Successors = map[uint32]Successor{}

		// For every potential successor, perform the least-upper bound with
		// existing superlocations in the analysis result.
		for _, succ := range s.GetTransitions(C, state) {
			// Retrieve the corresponding configuration for the given successor,
			// or insert it in the superlocation graph if not already present.
			s1 := G.GetOrSet(succ.Configuration())
			s1Loc := s1.Superloc
			// Add found successor to successor map, if not already present, and record
			// the added transition to the "state-less" successor map
			s.AddSuccessor(succ.DeriveConf(s1))

			// Retreive existing state in the analysis result for the
			// superlocation, or ⊥ if not found
			prevState := analysis.GetOrBot(s1Loc)
			updState := succ.State

			if s1.IsPanicked() {
				// Add the panicking superlocation to the analysis result,
				// but otherwise skip processing it.
				analysis = analysis.Update(s1Loc, updState)
				continue
			}

			// For previous state, σ, and updated state, σ', if σ ≠ σ ⊔ σ', then
			// update the abstract state at the superlocation s to σ ⊔ σ' and add
			// s to the worklist.
			if lub := updState.MonoJoin(prevState); !lub.Eq(prevState) {
				analysis = analysis.Update(s1Loc, lub)
				worklist.Add(s1)
			}
		}
	}

	return G, analysis
}
