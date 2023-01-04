package absint

import (
	"fmt"
	"go/types"
	"log"
	"time"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	T "github.com/cs-au-dk/goat/analysis/transition"
	"github.com/cs-au-dk/goat/utils/slices"

	"golang.org/x/tools/go/ssa"
)

// AbsConfiguration is a node in the superlocation graph. It connects
// a superlocation to its successor and predecessor relations,
type AbsConfiguration struct {
	defs.Superloc

	Successors   map[uint32]Successor
	predecessors map[uint32]*AbsConfiguration

	// panicked caches the result of the "IsPanicked" check
	panicked *bool
}

// Init initializes all the components of the abstract configuration e.g.,
// the predecessor and successor maps.
func (s *AbsConfiguration) Init() *AbsConfiguration {
	s.Superloc = defs.Create().Superloc(make(map[defs.Goro]defs.CtrLoc))
	s.predecessors = make(map[uint32]*AbsConfiguration)
	s.Successors = make(map[uint32]Successor)
	return s
}

// Copy creates a deep copy of a configuration, excluding
// relation information i.e., successors and predecessors.
func (s *AbsConfiguration) Copy() *AbsConfiguration {
	copy := Create().AbsConfiguration()

	copy.Superloc = s.Superloc
	return copy
}

// AddSuccessor adds a successor to the set of successors.
// Successors are de-duplicated by comparing with the the underlying superlocation,
// and the transition type.
func (s *AbsConfiguration) AddSuccessor(succ Successor) {
	s.Successors[succ.Hash()] = succ
}

// AddPredecessor adds a successor to the set of successors.
// Predecessors are de-duplicated by comparing with the the underlying superlocation,
// and the transition type.
func (s *AbsConfiguration) AddPredecessor(s1 *AbsConfiguration) {
	s.predecessors[s1.Hash()] = s1
}

// Derive updates the configuration with a new superlocation derived from the given
// map of threads.
func (s *AbsConfiguration) Derive(threads map[defs.Goro]defs.CtrLoc) *AbsConfiguration {
	s.Superloc = s.Superloc.Derive(threads)
	return s
}

// DeriveThread updates the configuration with a new superlocation where the given thread
// `tid` has been bound to the giben control location, `cl`.
func (s *AbsConfiguration) DeriveThread(tid defs.Goro, cl defs.CtrLoc) *AbsConfiguration {
	s.Superloc = s.Superloc.DeriveThread(tid, cl)
	return s
}

// PrettyPrint pretty prints the abstract configuration.
func (s *AbsConfiguration) PrettyPrint() {
	fmt.Println(s.String())
}

// IsCommunicating is true if no silent threads exist in the underlying superlocation
// of the abstract configuration.
func (s *AbsConfiguration) IsCommunicating(C AnalysisCtxt, state L.AnalysisState) bool {
	return s.nextSilentProgress(C, state) == nil
}

// IsPanicked returns true iff. the configuration contains a panicked goroutine.
func (s *AbsConfiguration) IsPanicked() bool {
	if s.panicked == nil {
		_, _, found := s.Find(func(_ defs.Goro, cl defs.CtrLoc) bool {
			return cl.Panicked()
		})
		s.panicked = &found
	}
	return *s.panicked
}

// isAtRelevantCommunicationNode returns whether the control location is a
// communication operation on a concurrency primitive that is focused (wrt. C).
func (s *AbsConfiguration) isAtRelevantCommunicationNode(
	C AnalysisCtxt, mem L.Memory,
	g defs.Goro, cl defs.CtrLoc,
) bool {
	node := cl.Node()
	if !node.IsCommunicationNode() {
		// Non-communication nodes are automatically not relevant.
		return false
	} else if _, isTerminated := node.(*cfg.TerminateGoro); isTerminated ||
		C.FocusedPrimitives == nil {
		// Goroutine termination nodes are not relevant.
		return true
	}

	// For every communication operand at the node, check for wildcards
	for _, param := range cfg.CommunicationPrimitivesOf(node) {
		av, mem := C.swapWildcard(s.Superloc, g, mem, param)
		nonNilLocs := av.PointerValue().FilterNil().Entries()

		if _, isChan := param.Type().Underlying().(*types.Chan); isChan && len(nonNilLocs) == 0 {
			if !av.PointerValue().Contains(loc.NilLocation{}) {
				panic(fmt.Errorf("Expected %v to contain a nil pointer, was empty?", param))
			}

			// If any channel primitive is nil we treat the operation as relevant.
			// This simplifies the implementation of the "silent" abstract
			// interpreter as it does not have to reason about which operations are
			// enabled and whether blocking is possible
			// (which it is not possible to model there currently).
			return true
		}

		if _, anyFocused := slices.Find(nonNilLocs, func(l loc.Location) bool {
			return C.IsLocationFocused(l, mem)
		}); anyFocused {
			return true
		}
	}

	return false
}

// nextSilentProgress finds a thread that is not at a communication node and
// returns it, or nil if there no such thread is found. Prefers ancestors over
// children and siblings are ordered by index. Other relations are ordered by hash values.
func (s *AbsConfiguration) nextSilentProgress(C AnalysisCtxt, state L.AnalysisState) (ret defs.Goro) {
	s.ForEach(func(g defs.Goro, cl defs.CtrLoc) {
		// Discard communication nodes and panicked control locations.
		if cl.Panicked() || s.isAtRelevantCommunicationNode(C, state.Memory(), g, cl) {
			return
		}

		good := ret == nil || g.IsParentOf(ret)
		if !good && !g.IsChildOf(ret) {
			if g.WeakEqual(ret) {
				good = g.Index() < ret.Index()
			} else {
				good = g.String() < ret.String()
			}
		}

		if good {
			ret = g
		}
	})
	return ret
}
func (s *AbsConfiguration) GetTransitions(
	C AnalysisCtxt,
	initState L.AnalysisState) transfers {

	// Determine whether any thread should be progressed silently.
	if progressSilently := s.nextSilentProgress(C, initState); progressSilently != nil {
		C.Log.Superloc = s.Superloc
		return s.GetSilentSuccessors(C, progressSilently, initState)
	}

	// leaves contains the cfg nodes the different goroutines can end up at (without synchronizing).
	leaves := make(map[defs.Goro]map[defs.CtrLoc]struct{})

	// Update leaves when communicating
	updateLeavesComm := func(v defs.CtrLoc, g defs.Goro) {
		leaves[g][v] = struct{}{}
	}

	mops := L.MemOps(initState.Memory())

	// Populate the leaves maps
	s.ForEach(func(g defs.Goro, cl defs.CtrLoc) {
		leaves[g] = make(map[defs.CtrLoc]struct{})

		switch n := cl.Node().(type) {
		case *cfg.Select:
			for _, op := range n.Ops() {
				ls, mem := C.getCommunicationLeaves(s.Superloc, g, mops.Memory(), cl.Derive(op))
				if mem.Lattice() == nil {
					fmt.Println(ls)
				}
				mops = L.MemOps(mem)
				for w := range ls {
					updateLeavesComm(w, g)
				}
			}
		default:
			ls, mem := C.getCommunicationLeaves(s.Superloc, g, mops.Memory(), cl)
			mops = L.MemOps(mem)
			for w := range ls {
				updateLeavesComm(w, g)
			}
		}
	})

	// Check whether the root thread has progressed to termination.
	// If so, cut off abstract interpretation here.
	// if _, ok := s.Superloc.GetUnsafe(s.Superloc.Main()).Node().(*cfg.TerminateGoro); ok {
	// 	return nil
	// }

	if mops.Memory().Lattice() == nil {
		log.Fatal("Memory is nil?", mops)
	}
	// Find communication partners and other transitions
	return s.GetCommSuccessors(C, leaves, initState.UpdateMemory(mops.Memory()))
}

func (s *AbsConfiguration) IntraprocessualFixpoint(
	C AnalysisCtxt,
	g defs.Goro,
	initState L.AnalysisState,
) (
	analysis map[defs.CtrLoc]L.AnalysisState,
	graph map[defs.CtrLoc][]defs.CtrLoc,
	steps int,
) {
	// Intra-processual analysis worklist.
	cl0 := s.GetUnsafe(g)
	analysis = map[defs.CtrLoc]L.AnalysisState{cl0: initState}

	if cl0.Panicked() {
		panic("Abstract interpretation of panicked control locations is disabled")
	}

	graph = map[defs.CtrLoc][]defs.CtrLoc{}
	// NOTE: You can visualize this graph with `VisualizeIntraprocess(g, graph, analysis)`
	/* defer func() {
		if err := recover(); err != nil || (C.Metrics.Enabled() && C.Metrics.Outcome == OUTCOME_PANIC) {
			VisualizeIntraprocess(g, graph, analysis)
			if err != nil {
				panic(err)
			}
		}
	}() */

	// When to stop looking for successors.
	// Currently when encountering a communication node, a spawn of a goroutine,
	// or a panicked control location.
	stopCond := func(cl defs.CtrLoc) bool {
		n := cl.Node()
		if s.isAtRelevantCommunicationNode(C, analysis[cl].Memory(), g, cl) {
			return true
		}

		if len(n.Spawns()) == 0 {
			return false
		}

		return true
		/*
			// The next available index for a goroutine spawn.
			index := s.NextIndex(
				// Prevent cyclical spawns in goroutines.
				g.Spawn(cl).GetRadix())
			// Only stop at spawn if we will actually spawn a goroutine
			return opts.WithinGoroBound(index)
		*/
	}

	checkSkippedInterval := 100_000

	W := defs.EmptyIntraprocessWorklist(C.LoadRes.CtrLocPriorities)
	W.Add(cl0)
FIXPOINT:
	for ; !W.IsEmpty(); steps++ {
		if C.Metrics.Enabled() && steps%checkSkippedInterval == 0 && steps > 0 {
			select {
			case <-C.skipped:
				break FIXPOINT
			default:
			}
		}

		cl := W.GetNext()
		pair := analysis[cl]

		if stopCond(cl) {
			continue
		}

		edges := []defs.CtrLoc{}
		s.singleSilent(C, g, cl, pair).ForEach(func(cl defs.CtrLoc, updPair L.AnalysisState) {
			edges = append(edges, cl)

			if cl.Panicked() {
				// Do not spend time joining memory for panicked control locations.
				// We will never use this memory, and joining can be expensive due to the high
				// number of predecessors (and therefore different versions of the memory).
				analysis[cl] = updPair
				return
			}

			// If we previously visited w, we join the memory there with the updated memory and
			// check for a change to determine if we should push the location into the queue again.
			// If it is the first time we encounter w, we always push it.
			push := true
			if prevPair, ok := analysis[cl]; ok && len(cl.Node().Predecessors()) != 1 {
				// NOTE: We can (maybe) save some joins by computing the number of predecessors
				// ourselves (like we compute the forward edges for `graph`). The number we
				// find can possibly be lower than what is in the CFG (for example for branches
				// that we can always correctly predict).
				updPair = prevPair.MonoJoin(updPair)
				push = !updPair.Eq(prevPair)
			}

			if push {
				analysis[cl] = updPair
				W.Add(cl)
			}
		})

		graph[cl] = edges
	}

	return
}

// Returns possible multi-step silent successors for the given thread.
// Uses the abstract interpretation framework to model the effects of single steps on the way.
// (This method contains an internal fixpoint computation.)
func (s *AbsConfiguration) GetSilentSuccessors(
	C AnalysisCtxt,
	g defs.Goro,
	initState L.AnalysisState,
) transfers {
	cl0 := s.GetUnsafe(g)
	start := time.Now()
	analysis, graph, steps := s.IntraprocessualFixpoint(C, g, initState)
	_ = graph

	duration := time.Since(start)
	reanalysisFactor := float64(steps) / float64(len(analysis))
	if duration >= 2*time.Second && reanalysisFactor >= 20. {
		log.Printf("Slow (%s) internal transition for %v from %v of %v", duration, g, cl0, cl0.Node().Function())
		log.Printf("Made %d steps to reach fixpoint for %d locations (%.2f steps/loc, %.2f steps/s)",
			steps, len(analysis), float64(steps)/float64(len(analysis)), float64(steps)/duration.Seconds())
	}

	results := make(transfers)
	addResult := func(conf *AbsConfiguration, state L.AnalysisState) {
		results.succUpdate(Successor{
			configuration: conf.Copy(),
			transition:    T.NewIn(g),
		}, state)
	}

	addTermination := func(cl defs.CtrLoc, state L.AnalysisState, cause int) {
		addResult(
			s.Copy().DeriveThread(g,
				cl.Derive(
					C.LoadRes.Cfg.AddSynthetic(cfg.SynthConfig{
						Type:             cfg.SynthTypes.TERMINATE_GORO,
						Function:         cl.Node().Function(),
						TerminationCause: cause,
					}))),
			state)
	}

	// Extract proper successors and handle interesting spawns and potential goroutine exit
	for cl, state := range analysis {
		n := cl.Node()
		switch {
		case cl.Panicked():
			fallthrough
		case s.isAtRelevantCommunicationNode(C, state.Memory(), g, cl):
			addResult(
				s.Copy().DeriveThread(g, cl),
				state)
		case len(n.Spawns()) > 0:
			radix := g.Spawn(cl).GetRadix()
			// The next available index for a goroutine spawn.
			// Prevent cyclical spawns in goroutines.
			index := s.NextIndex(radix)

			// If the next index is less than the goroutine bound,
			// add a goroutine with that index. If the goroutine bound was exceeded,
			// pretend that the spawn is a no-op.
			// TODO: Unsound
			if !opts.WithinGoroBound(index) && C.Log.Enabled {
				log.Println("Tried spawning", g.Spawn(cl), "in excess of goroutine bound", index, "at superlocation", s)
			}

			spawnee := radix.SetIndex(index)
			C.CheckMaxSuperloc(s.Superloc, spawnee)
			callIns := n.(*cfg.SSANode).Instruction().(*ssa.Go)

			paramTransfers, mayPanic := C.transferParams(
				s.Superloc,
				callIns.Call,
				g, spawnee, state.Memory(),
			)

			if mayPanic {
				addResult(s.Copy().DeriveThread(g, cl.Panic()), state)
			}

			blacklists := make(map[*ssa.Function]struct{})
			for entry := range n.Spawns() {
				newMem, found := paramTransfers[entry.Function()]
				switch entry.(type) {
				case *cfg.BuiltinCall:
					// Builtins are special...
					newMem, found = paramTransfers[nil]
				case *cfg.APIConcBuiltinCall:
					newMem := state.Memory()
					for _, arg := range callIns.Call.Args {
						// Skip constants, they don't need to be transferred (and they don't have a location)
						if _, ok := arg.(*ssa.Const); !ok {
							newMem = newMem.Update(
								loc.LocationFromSSAValue(spawnee, arg),
								EvaluateSSA(g, state.Memory(), arg),
							)
						}
					}

					if g.Length() >= spawnee.Length() {
						if !opts.NoAbort() {
							C.Panic(fmt.Errorf("%w: recursion leads to %v", ErrUnboundedGoroutineSpawn, g.Spawn(cl)))
						}
						blacklists[nil] = struct{}{}
						continue
					}
					if !opts.WithinGoroBound(index) {
						if !opts.NoAbort() {
							C.Panic(
								fmt.Errorf(
									"%w: control flow cycle to %v (%s)",
									ErrUnboundedGoroutineSpawn,
									g.SpawnIndexed(cl, index),
									cl.PosString(),
								),
							)
						}
						blacklists[nil] = struct{}{}
						continue
					}
					addResult(
						s.Copy().DeriveThread(spawnee,
							defs.Create().CtrLoc(
								entry,
								entry.Block().Parent(),
								false),
						).DeriveThread(g, cl.Successor()),
						state.UpdateMemory(newMem))
					continue
				}

				if !found {
					// Skip any kind of handling for spawns that the
					// abstract interpreter knows cannot occur.
					continue
				}

				// If the spawnee is not a blacklisted function,
				// then spawn it normally.
				if !C.Blacklisted(callIns, entry.Function()) {
					C.Metrics.AddGo(cl)
					C.Metrics.ExpandFunction(entry.Function())

					if g.Length() >= spawnee.Length() {
						if !opts.NoAbort() {
							C.Panic(fmt.Errorf("%w: recursion leads to %v", ErrUnboundedGoroutineSpawn, g.Spawn(cl)))
						}
						blacklists[entry.Function()] = struct{}{}
						continue
					}
					if !opts.WithinGoroBound(index) {
						if !opts.NoAbort() {
							C.Panic(
								fmt.Errorf(
									"%w: control flow cycle to %v (%s)",
									ErrUnboundedGoroutineSpawn,
									g.SpawnIndexed(cl, index),
									cl.PosString(),
								),
							)
						}
						blacklists[entry.Function()] = struct{}{}
						continue
					}
					addResult(
						s.Copy().DeriveThread(spawnee,
							defs.Create().CtrLoc(
								entry,
								entry.Function(),
								false),
						).DeriveThread(g, cl.Successor()),
						state.UpdateMemory(newMem))
				} else {
					blacklists[entry.Function()] = struct{}{}
				}
			}

			if len(blacklists) > 0 {
				// Otherwise, do not create a goroutine, and just top-inject
				// the parameters into the analysis state.
				mem := C.TopInjectParams(callIns, g, state, blacklists)
				addResult(
					s.Copy().DeriveThread(g, cl.Successor()),
					state.UpdateMemory(mem))
			}
		default:
			if _, ok := n.(*cfg.FunctionExit); ok && n.Function() == cl.Root() {
				// If the function exit node belongs to the root function
				// of the goroutine, it indicates a potential goroutine exit point.
				addTermination(cl, state, cfg.GoroTermination.EXIT_ROOT)
			}
		}
	}

	// If we did not find any interesting points to stop at, add a synthetic goroutine
	// termination point to prevent us from picking this goroutine again.
	// TODO: Use some form of cycle detection to determine if the goroutine can
	// loop forever.
	const (
		HAS_SUCCS = 1 << iota
		HAS_PANICS
	)
	addTerm := 0
	for _, succ := range results {
		if !succ.Configuration().GetUnsafe(g).Panicked() {
			addTerm = addTerm | HAS_SUCCS
			break
		} else {
			addTerm = addTerm | HAS_PANICS
		}
	}

	if addTerm == 0 {
		addTermination(cl0, initState, cfg.GoroTermination.INFINITE_LOOP)
	}

	return results
}
