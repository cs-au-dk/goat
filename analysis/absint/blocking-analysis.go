package absint

import (
	"fmt"
	"log"

	"github.com/cs-au-dk/goat/analysis/absint/ops"
	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/analysis/transition"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/graph"

	"github.com/fatih/color"

	"golang.org/x/tools/go/ssa"
)

type Blocks map[defs.Superloc]map[defs.Goro]struct{}

func (o Blocks) String() string {
	str := "\n"
	for sl, gs := range o {
		if len(gs) == 0 {
			continue
		}

		str += "Potential blocked goroutine at superlocation: " + sl.String() + "\n"
		for g := range gs {
			cl := sl.GetUnsafe(g)
			str += fmt.Sprintf("Goroutine: %s\nControl location: %s\nSource: %s\n",
				g, cl, g.CtrLoc().Root().Prog.Fset.Position(cl.Node().Pos()),
			)
		}
		str += "\n"
	}

	return str
}

func (o Blocks) PrintPath(G SuperlocGraph, A L.Analysis, g graph.Graph[*ssa.Function]) {
	// Print shortest path to blocking configuration
	for sl := range o {

		type link struct {
			transition transition.Transition
			sl         defs.Superloc
		}

		preds := make(map[defs.Superloc]link)

		G.ToGraph().BFS(G.Entry(), func(next *AbsConfiguration) bool {
			nextSl := next.superloc

			if sl.Equal(nextSl) {
				return true
			}

			for _, succ := range next.Successors {
				if _, ok := preds[succ.configuration.superloc]; !ok {
					preds[succ.configuration.superloc] = link{
						succ.transition,
						nextSl,
					}
				}
			}

			return false
		})

		path := []link{{nil, sl}}

		for pred, ok := preds[sl]; ok; pred, ok = preds[pred.sl] {
			path = append(path, pred)
		}

		fmt.Println(color.RedString("Blocking path:"))
		for i := len(path) - 1; i >= 0; i-- {
			chMem := L.MemOps(A.GetUnsafe(path[i].sl).Memory().Channels())
			printCh := func(c loc.Location) {
				if ch, ok := chMem.Get(c); ok {
					fmt.Println("|", ch)
				}
			}

			// chMem.Memory().ForEach(func(al loc.AddressableLocation, av L.AbstractValue) {
			// 	fmt.Println(al)
			// 	fmt.Println(al.Position())
			// 	fmt.Println(av)
			// })
			fmt.Println(path[i].sl.StringWithPos())
			if t := path[i].transition; t != nil {
				fmt.Println("|")
				fmt.Println("|>", t)

				switch t := t.(type) {
				case transition.Sync:
					fmt.Println("|-- at", t.Channel.Position())
					printCh(t.Channel)
					fmt.Println("|")
				case transition.Send:
					fmt.Println("|-- at", t.Chan.Position())
					printCh(t.Chan)
					fmt.Println("|")
				case transition.Receive:
					fmt.Println("|-- at", t.Chan.Position())
					printCh(t.Chan)
					fmt.Println("|")
				case transition.In:
					// Get the shortest callee path for internal transitions:
					from := path[i].sl.GetUnsafe(t.Progressed()).Node().Function()
					to := path[i-1].sl.GetUnsafe(t.Progressed()).Node().Function()

					fpreds := map[*ssa.Function]*ssa.Function{}

					g.BFS(from, func(f *ssa.Function) bool {
						if f == to {
							return true
						}

						for _, e := range g.Edges(f) {
							if _, ok := fpreds[e]; !ok && e != from {
								fpreds[e] = f
							}
						}

						return false
					})

					path := []*ssa.Function{to}
					for pred, ok := fpreds[to]; ok; pred, ok = fpreds[pred] {
						path = append(path, pred)
					}

					for i := len(path) - 1; i >= 0; i-- {
						fmt.Println("  |", path[i])
						fmt.Println("   ", path[i].Prog.Fset.Position(path[i].Pos()))
					}
					fmt.Println("|")
				}

				utils.Prompt()
			}
		}
	}
}

func (o Blocks) Log() {
	fmt.Println(o.String())
}

func (o Blocks) register(sl defs.Superloc, g defs.Goro) {
	if _, found := o[sl]; !found {
		o[sl] = make(map[defs.Goro]struct{})
	}
	o[sl][g] = struct{}{}
}

func (o Blocks) ForEach(do func(sl defs.Superloc, gs map[defs.Goro]struct{})) {
	for sl, gs := range o {
		do(sl, gs)
	}
}

func (o Blocks) Exists(pred func(sl defs.Superloc, gs map[defs.Goro]struct{}) bool) bool {
	_, _, found := o.Find(pred)
	return found
}

func (o Blocks) Find(pred func(sl defs.Superloc, gs map[defs.Goro]struct{}) bool) (defs.Superloc, map[defs.Goro]struct{}, bool) {
	for sl, gs := range o {
		if pred(sl, gs) {
			return sl, gs, true
		}
	}

	return defs.Superloc{}, nil, false
}

// UpdateWith joins the results of `other` into `o`.
func (o Blocks) UpdateWith(other Blocks) {
	for sl, ogs := range other {
		if gs, found := o[sl]; !found {
			o[sl] = ogs
		} else {
			for g, v := range ogs {
				gs[g] = v
			}
		}
	}
}

// Blocked goroutine analysis mode
const (
	// "May" mode overapproximates the possiblity of a blocked goroutine.
	// This mode will register a potential orphan if any path might lead to orphanage.
	MAY = iota
	// "Must" mode underapproximates the possibility of a blocked goroutine.
	// In this mode, potential blocks are not considered if they have at least
	// one "non-orphaning" future. If all futures point to orphanage,
	MUST
)

func BlockAnalysis(C AnalysisCtxt, G SuperlocGraph, result L.Analysis) Blocks {
	return BlockAnalysisFiltered(C, G, result, false)
}

func BlockAnalysisFiltered(
	C AnalysisCtxt,
	G SuperlocGraph,
	result L.Analysis,
	// If true, will only report bugs at control locations where the following
	// condition holds: the primitive used at that location (according to the
	// upfront pointer analysis) must be one of the focused primitives.
	// Note that blocks from sending or receiving on definitely nil channels
	// (again according to the upfront pointer analysis) will not be reported.
	filterWithPSet bool,
) (res Blocks) {
	res = make(Blocks)
	// mode := MAY

	// var CheckForOrphans func(*AbsConfiguration, *immutable.Map) bool
	// CheckForOrphans = func(conf *AbsConfiguration, visited *immutable.Map) bool {
	// 	// If the configuration is terminal, if no channel operations are
	// 	// found at the superlocation, there is definitely no orphan.
	// 	if len(conf.GetSuccessorMap()) == 0 {
	// 		sl := conf.Superlocation()
	// 		gs, found := sl.FindAll(func(g defs.Goro, cl defs.CtrLoc) bool {
	// 			// Check that no control location is a stagnated
	// 			return cl.Node().IsChannelOp()
	// 		})
	// 		for g, _ := range gs {
	// 			orphans.register(sl, g)
	// 		}
	// 		return found
	// 	}

	// 	// Otherwise, check for the orphans in the successors
	// 	res := mode == MAY
	// 	for _, succ := range conf.GetSuccessorMap() {
	// 		succRes := CheckForOrphans(
	// 			succ.Configuration(),
	// 			visited.Set(conf.Superlocation(), struct{}{}))
	// 		switch mode {
	// 		case MAY:
	// 			res = res && succRes
	// 		case MUST:
	// 			res = res || succRes
	// 		}
	// 	}

	// }
	dedup := map[defs.Goro]map[defs.CtrLoc]struct{}{}

	isInContext := func(cl defs.CtrLoc) bool {
		pkg := cl.Node().Function().Pkg
		return pkg != nil && pkg.Pkg.Path() == "context"
	}

	isTerminated := func(cl defs.CtrLoc) bool {
		_, terminated := cl.Node().(*cfg.TerminateGoro)
		return terminated
	}

	transitionSystem := G.ToGraph()

	/*
		Compute which configurations lead to guaranteed panics. We don't want
		to report blocking bugs in such configurations as they are probably
		false positives.
		Configurations in a component lead to a definite panic if all transitions
		that leave the component lead to definite panics.
		Otherwise, if no edges leave the component, and if the component
		contains only a single configuration:
		* Silent conf must panic, otherwise there would be edges to other components
		* Sync conf guaranteed to panic if either condition holds:
			- Close of or send on definitely closed channel
			- Close of definitely nil channel
	*/

	// Utility method for checking whether the specified synchronizing
	// goroutine will definitely panic.
	definiteCommPanic := func(sl defs.Superloc, g defs.Goro) bool {
		st, cl := result.GetUnsafe(sl), sl.GetUnsafe(g)
		var toCheck []ssa.Value
		var isClose bool
		switch n := cl.Node().(type) {
		case *cfg.Select:
			for _, op := range n.Ops() {
				if s, ok := op.(*cfg.SelectSend); ok {
					toCheck = append(toCheck, s.Channel())
				}
			}
		case *cfg.SSANode:
			if s, ok := n.Instruction().(*ssa.Send); ok {
				toCheck = append(toCheck, s.Chan)
			}
		case *cfg.BuiltinCall:
			if n.Builtin().Name() == "close" {
				isClose = true
				toCheck = append(toCheck, n.Channel())
			}
		}

		for _, chn := range toCheck {
			av, mem := C.swapWildcard(g, st.Memory(), chn)
			chV := L.Consts().BotValue()
			ops.ToDeref(av).OnSucceed(func(av L.AbstractValue) {
				chV = ops.Load(av, mem)
			})

			// Trying to send on or close already closed channel - definite panic
			if (!chV.IsBot() && chV.ChanValue().Status().Is(false)) ||
				// Closing a definitely nil channel is also a definite panic
				(isClose && av.PointerValue().Eq(L.Consts().PointsToNil())) {
				return true
			}
		}

		return false
	}

	scc := transitionSystem.SCC([]*AbsConfiguration{G.Entry()})
	guaranteedPanic := make([]bool, len(scc.Components))
	for ci, comp := range scc.Components {
		bad := false
		edgesOut := scc.ToGraph().Edges(ci)
		if len(edgesOut) > 0 {
			bad = true
			for _, cj := range edgesOut {
				if !guaranteedPanic[cj] {
					bad = false
					break
				}
			}
		} else if len(comp) == 1 {
			conf := comp[0]
			st := result.GetUnsafe(conf.superloc)
			if !conf.IsSynchronizing(C, st) {
				if len(conf.Successors) == 0 {
					panic(fmt.Errorf("Expected non-synchronizing configuration %v to have successors!", conf))
				}

				// Only panic transitions are possible
				bad = true
			} else {
				// Skip if one of the goroutines is guaranteed to panic due to
				// closing or sending on channels that are all _definitely_ closed.
				_, _, bad = conf.Superlocation().Find(func(g defs.Goro, _ defs.CtrLoc) bool {
					return definiteCommPanic(conf.Superlocation(), g)
				})
			}
		} /* else {
			// TODO: It's not clear to me how we want to handle components with
			// multiple configurations without any edge leaving the component.
		} */

		guaranteedPanic[ci] = bad
	}

	G.ForEach(func(conf *AbsConfiguration) {
		analysis := result.GetUnsafe(conf.Superlocation())

		// Skip checking for orphans in configurations where a goroutine has
		// panicked and in non-synchronizing configurations.
		if conf.IsPanicked() || guaranteedPanic[scc.ComponentOf(conf)] || !conf.IsSynchronizing(C, analysis) {
			return
		}

		// Filter out blocking bugs in configurations where a goroutine has
		// deadlocked in the context package as these are (most likely) false positives
		if _, _, deadLockInContext := conf.Superlocation().Find(func(g defs.Goro, cl defs.CtrLoc) bool {
			return isInContext(cl) && !isTerminated(cl) && !mayProgress(transitionSystem, conf, g)
		}); deadLockInContext {
			return
		}

		// Check if there is a goroutine at a communication operation which can never progress
		conf.ForEach(func(g defs.Goro, cl defs.CtrLoc) {
			// Terminated goroutines are not buggy
			if isTerminated(cl) {
				return
			}

			// Don't report if the goroutine has no progress because it's guaranteed to panic
			if definiteCommPanic(conf.Superlocation(), g) {
				return
			}

			// Only report each goroutine/control location combination once.
			prevFound, ok := dedup[g]
			if !ok {
				dedup[g] = map[defs.CtrLoc]struct{}{}
				prevFound = dedup[g]
			}

			if _, found := prevFound[cl]; found {
				return
			}

			if filterWithPSet {
				// Check if a focused primitive flows here.
				ok := false
			OUT:
				for _, reg := range cfg.CommunicationPrimitivesOf(cl.Node()) {
					for _, lab := range C.LoadRes.Pointer.Queries[reg].PointsTo().Labels() {
						if C.IsPrimitiveFocused(lab.Value()) {
							ok = true
							break OUT
						}
					}
				}

				if !ok {
					return
				}
			}

			if !mayProgress(transitionSystem, conf, g) {
				for _, prim := range cfg.CommunicationPrimitivesOf(cl.Node()) {
					av, _ := C.swapWildcard(g, analysis.Memory(), prim)
					if av.PointerValue().Eq(L.Consts().PointsToNil()) {
						log.Printf("Potential false positive in %v: %v = {nil}\n", cl.Node(), prim.Name())
					}
				}

				res.register(conf.Superlocation(), g)
				prevFound[cl] = struct{}{}
			}
		})
	})

	return
}

// Returns true iff. there exists a transitive successor configuration to `conf`
// where goroutine `g` has progressed.
func mayProgress(G graph.Graph[*AbsConfiguration], conf *AbsConfiguration, g defs.Goro) bool {
	return G.BFS(conf, func(cur *AbsConfiguration) bool {
		return !conf.GetUnsafe(g).Equal(cur.GetUnsafe(g))
	})
}
