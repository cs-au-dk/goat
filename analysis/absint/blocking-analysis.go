package absint

import (
	"Goat/analysis/absint/ops"
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
	"Goat/analysis/transition"
	"Goat/utils"
	"Goat/utils/graph"
	"fmt"

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

func (o Blocks) PrintPath(G SuperlocGraph, A L.Analysis, C utils.SSAValueSet) {
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
				}

				utils.Prompt()
			}
		}

		if len(path) > 0 {
			fmt.Println(A.GetUnsafe(path[0].sl).Memory().ForChannels(C))
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

func BlockAnalysis(C AnalysisCtxt, G SuperlocGraph, result L.Analysis) (res Blocks) {
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

	G.ForEach(func(conf *AbsConfiguration) {
		analysis := result.GetUnsafe(conf.Superlocation())

		// Skip checking for orphans in configurations where a goroutine has
		// panicked and in non-synchronizing configurations.
		if conf.IsPanicked() || !conf.IsSynchronizing(C, analysis) {
			return
		}

		// Filter out blocking bugs in configurations where a goroutine has
		// deadlocked in the context package as these are (most likely) false positives
		if _, _, deadLockInContext := conf.Superlocation().Find(func(g defs.Goro, cl defs.CtrLoc) bool {
			return isInContext(cl) && !isTerminated(cl) && !mayProgress(transitionSystem, conf, g)
		}); deadLockInContext {
			return
		}

		// Also skip if one of the goroutines is guaranteed to panic due to
		// closing or sending on channels that are all _definitely_ closed.
		mem := analysis.Memory()
		if _, _, shouldSkip := conf.Superlocation().Find(func(g defs.Goro, cl defs.CtrLoc) bool {
			var toCheck []ssa.Value
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
					toCheck = append(toCheck, n.Channel())
				}
			}

			for _, chn := range toCheck {
				av, mem := C.swapWildcard(g, mem, chn)
				chV := L.Consts().BotValue()
				ops.ToDeref(av).OnSucceed(func(av L.AbstractValue) {
					chV = ops.Load(av, mem)
				})

				if !chV.IsBot() && chV.ChanValue().Status().Is(false) {
					// Trying to send on or close already closed channel - definite panic
					return true
				}
			}

			return false
		}); shouldSkip {
			return
		}

		// Check if there is a goroutine at a communication operation which can never progress
		conf.ForEach(func(g defs.Goro, cl defs.CtrLoc) {
			// Terminated goroutines are not buggy
			if isTerminated(cl) {
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

			if !mayProgress(transitionSystem, conf, g) {
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
