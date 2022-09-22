package absint

import (
	"fmt"
	"log"
	"time"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	T "github.com/cs-au-dk/goat/analysis/transition"

	"golang.org/x/tools/go/ssa"
)

// An abstract thread involves a CFG node and a root function indicating
// the function that was called when spawning the goroutine. Upon exiting
// the root function, a possible successor would be the termination of the
// goroutine. Together, they form an abstract control location.
type AbsCtrlLoc struct {
	cfg.Node
	Root *ssa.Function
}

// An abstract configuration involves a super location, (optionally)
// bookkeeping of the targeted thread, and other path conditions, e. g.
// channel status in terms of closing.
type AbsConfiguration struct {
	BaseConfiguration
	predecessors map[uint32]*AbsConfiguration
	superloc     defs.Superloc
	Target       defs.Goro
	level        ABSTRACTION_LEVEL
	panicked     *bool // cached result of IsPanicked
}

func (s *AbsConfiguration) Init(abs ABSTRACTION_LEVEL) Configuration {
	s.superloc = defs.Create().Superloc(make(map[defs.Goro]defs.CtrLoc))
	s.predecessors = make(map[uint32]*AbsConfiguration)
	s.Successors = make(map[uint32]Successor)
	if abs == ABS_CONCRETE {
		log.Fatal("Defined abstract configuration at concrete abstraction level.")
	}
	s.level = abs
	return s
}

func (s *AbsConfiguration) AbstractionLevel() ABSTRACTION_LEVEL {
	return s.level
}

func (s *AbsConfiguration) Main() defs.Goro {
	return s.superloc.Main()
}

// Create deep copy of configurations
func (s *AbsConfiguration) Copy() *AbsConfiguration {
	copy := Create().AbsConfiguration(s.level)
	copy.Target = s.Target

	copy.superloc = s.superloc
	return copy
}

func (s *AbsConfiguration) Hash() uint32 {
	return s.superloc.Hash()
}

func (s *AbsConfiguration) ForEach(do func(defs.Goro, defs.CtrLoc)) {
	s.superloc.ForEach(do)
}

func (s *AbsConfiguration) String() string {
	return s.superloc.String()
}

func (s *AbsConfiguration) Threads() defs.Superloc {
	return s.superloc
}

func (s *AbsConfiguration) Get(g defs.Goro) (defs.CtrLoc, bool) {
	return s.superloc.Get(g)
}

func (s *AbsConfiguration) GetUnsafe(g defs.Goro) defs.CtrLoc {
	return s.superloc.GetUnsafe(g)
}

func (s *AbsConfiguration) AddPredecessor(s1 *AbsConfiguration) {
	s.predecessors[s1.Hash()] = s1
}

// Statefully set the threads in the given configuration.
// Usage recommended only on expendable deep copies of configurations.
func (s *AbsConfiguration) SetThreads(threads defs.Superloc) {
	s.superloc = threads
}

// Derive new superlocation for given configuration.
func (s *AbsConfiguration) Derive(threads map[defs.Goro]defs.CtrLoc) *AbsConfiguration {
	s.superloc = s.superloc.Derive(threads)
	return s
}

func (s *AbsConfiguration) DeriveThread(tid defs.Goro, cl defs.CtrLoc) *AbsConfiguration {
	s.superloc = s.superloc.DeriveThread(tid, cl)
	return s
}

func (s *AbsConfiguration) Superlocation() defs.Superloc {
	return s.superloc
}

// Coarse configuration cannot be abstracted further.
// Acts as the identity function if given the coarse abstraction level.
func (s *AbsConfiguration) Abstract(abstractTo ABSTRACTION_LEVEL) Configuration {
	if abstractTo < ABS_COARSE {
		log.Fatal("Invalid abstraction: attempted abstraction from level ", ABS_COARSE, " to ", abstractTo)
	}
	return s
}

func (s *AbsConfiguration) PrettyPrint() {
	fmt.Println(s.superloc.String())
}

func (s *AbsConfiguration) IsSynchronizing(C AnalysisCtxt, state L.AnalysisState) bool {
	return s.nextSilentProgress(C, state) == nil
}

// Returns true iff. the configuration contains a panicked goroutine
func (s *AbsConfiguration) IsPanicked() bool {
	if s.panicked == nil {
		_, _, found := s.Superlocation().Find(func(_ defs.Goro, cl defs.CtrLoc) bool {
			return cl.Panicked()
		})
		s.panicked = &found
	}
	return *s.panicked
}

// Returns whether the control location is a communication operation
// on a concurrency primitive that is focused (wrt. C).
func (s *AbsConfiguration) isAtRelevantCommunicationNode(
	C AnalysisCtxt, mem L.Memory,
	g defs.Goro, cl defs.CtrLoc,
) bool {
	node := cl.Node()
	if !node.IsCommunicationNode() {
		return false
	} else if _, isTerminated := node.(*cfg.TerminateGoro); isTerminated ||
		C.FocusedPrimitives == nil {
		return true
	}

	for _, param := range cfg.CommunicationPrimitivesOf(node) {
		av, _ := C.swapWildcard(g, mem, param)
		for _, l := range av.PointerValue().FilterNil().Entries() {
			// Dig out the allocation site location in case of field pointers
			for {
				switch lt := l.(type) {
				case loc.GlobalLocation:
				case loc.AllocationSiteLocation:
				case loc.FieldLocation:
					l = lt.Base
					continue
				default:
					log.Fatalf("%v %T", lt, lt)
					panic("???")
				}

				break
			}

			if site, ok := l.GetSite(); ok && C.IsPrimitiveFocused(site) {
				return true
			} else if !ok {
				log.Fatalf("%v has no site?", l)
			}
		}
	}

	return false
}

// Finds a thread that is not at a communication node and returns it.
// Returns nil if there is no such thread.
// Prefers ancestors over children and siblings are ordered by index.
// Other relations are ordered by hash values.
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

type getSuccResult = struct {
	Successor
	State L.AnalysisState
}

func (s *AbsConfiguration) GetTransitions(
	C AnalysisCtxt,
	initState L.AnalysisState) transfers {

	// Determine whether any thread should be progressed silently.
	if progressSilently := s.nextSilentProgress(C, initState); progressSilently != nil {
		C.Log.Superloc = s.superloc
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
				ls, mem := C.computeCommunicationLeaves(g, mops.Memory(), cl.Derive(op))
				if mem.Lattice() == nil {
					fmt.Println(ls)
				}
				mops = L.MemOps(mem)
				for w := range ls {
					updateLeavesComm(w, g)
				}
			}
		default:
			ls, mem := C.computeCommunicationLeaves(g, mops.Memory(), cl)
			mops = L.MemOps(mem)
			for w := range ls {
				updateLeavesComm(w, g)
			}
		}
	})

	// Check whether the root thread has progressed to termination.
	// If so, cut off abstract interpretation here.
	// if _, ok := s.superloc.GetUnsafe(s.superloc.Main()).Node().(*cfg.TerminateGoro); ok {
	// 	return nil
	// }

	if mops.Memory().Lattice() == nil {
		log.Fatal("Memory is nil?", mops)
	}
	// Find communication partners and other transitions
	return s.GetCommSuccessors(leaves, initState.UpdateMemory(mops.Memory()))
}

// Returns possible multi-step silent successors for the given thread.
// Uses the abstract interpretation framework to model the effects of single steps on the way.
// (This method contains an internal fixpoint computation.)
func (s *AbsConfiguration) GetSilentSuccessors(
	C AnalysisCtxt,
	g defs.Goro,
	initState L.AnalysisState,
) transfers {
	// Intra-processual analysis worklist.
	cl0 := s.Threads().GetUnsafe(g)
	analysis := map[defs.CtrLoc]L.AnalysisState{cl0: initState}

	if cl0.Panicked() {
		panic("Abstract interpretation of panicked control locations is disabled")
	}

	graph := map[defs.CtrLoc][]defs.CtrLoc{}
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
			index := s.Superlocation().NextIndex(
				// Prevent cyclical spawns in goroutines.
				g.Spawn(cl).GetRadix())
			// Only stop at spawn if we will actually spawn a goroutine
			return opts.WithinGoroBound(index)
		*/
	}

	start := time.Now()
	steps := 0
	checkSkippedInterval := 100_000

	W := defs.EmptyIntraprocessWorklist(C.LoadRes.CtrLocPriorities)
	W.Add(cl0)
FIXPOINT:
	for ; !W.IsEmpty(); steps++ {
		if C.Metrics.Enabled() && steps%checkSkippedInterval == 0 && steps > 0 {
			select {
			case <-C.Metrics.skipped:
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
			transition:    T.In{Progressed: g},
		}, state)
	}

	addTermination := func(cl defs.CtrLoc, state L.AnalysisState, cause int) {
		addResult(
			s.Copy().DeriveThread(g,
				cl.Derive(
					cfg.AddSynthetic(cfg.SynthConfig{
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
			index := s.Superlocation().NextIndex(
				// Prevent cyclical spawns in goroutines.
				radix)

			// If the next index is less than the goroutine bound,
			// add a goroutine with that index. If the goroutine bound was exceeded,
			// pretend that the spawn is a no-op.
			// TODO: Unsound
			if !opts.WithinGoroBound(index) {
				if C.Log.Enabled {
					log.Println("Tried spawning", g.Spawn(cl), "in excess of goroutine bound", index, "at superlocation", s)
				}
			}

			spawnee := radix.SetIndex(index)
			C.CheckMaxSuperloc(s.superloc, spawnee)
			callIns := n.(*cfg.SSANode).Instruction().(*ssa.Go)

			paramTransfers, mayPanic := C.transferParams(
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
								evaluateSSA(g, state.Memory(), arg),
							)
						}
					}

					if g.Length() >= spawnee.Length() {
						if !opts.NoAbort() {
							C.Metrics.Panic(fmt.Errorf("%w: recursion leads to %v", ErrUnboundedGoroutineSpawn, g.Spawn(cl)))
						}
						blacklists[nil] = struct{}{}
						continue
					}
					if !opts.WithinGoroBound(index) {
						if !opts.NoAbort() {
							C.Metrics.Panic(fmt.Errorf("%w: control flow cycle to %v", ErrUnboundedGoroutineSpawn, g.SpawnIndexed(cl, index)))
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
							C.Metrics.Panic(fmt.Errorf("%w: recursion leads to %v", ErrUnboundedGoroutineSpawn, g.Spawn(cl)))
						}
						blacklists[entry.Function()] = struct{}{}
						continue
					}
					if !opts.WithinGoroBound(index) {
						if !opts.NoAbort() {
							C.Metrics.Panic(fmt.Errorf("%w: control flow cycle to %v", ErrUnboundedGoroutineSpawn, g.SpawnIndexed(cl, index)))
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
