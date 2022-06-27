package upfront

import (
	"Goat/pkgutil"
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// Goro describes the structure of the transitive
// closure of a goroutine.
type Goro struct {
	Entry             *ssa.Function
	ChannelOperations map[ssa.Value]*ChannelOp
	SpawnedGoroutines []*Goro
	CrossPackageCalls []*Goro
}

func (g *Goro) String() (goroStr string) {
	goroStr = g.Entry.String()
	if len(g.SpawnedGoroutines) > 0 {
		goroStr += "\nSpawned goroutines:"
		for _, spawned := range g.SpawnedGoroutines {
			goroStr += " " + spawned.Entry.String() + ","
		}
	}
	if len(g.CrossPackageCalls) > 0 {
		goroStr += "\nCross-package Calls:"
		for _, cpc := range g.CrossPackageCalls {
			goroStr += " " + cpc.Entry.String() + ","
		}
	}
	for value, channelOp := range g.ChannelOperations {
		goroStr += fmt.Sprintf("\n%s: %s", value.Name(), channelOp)
	}
	goroStr += "\n"
	return
}

// Ensure channel operations are initialized for a given value
func (g *Goro) initOps(val ssa.Value) *ChannelOp {
	if _, ok := g.ChannelOperations[val]; !ok {
		g.ChannelOperations[val] = new(ChannelOp)
	}
	return g.ChannelOperations[val]
}

// ProcessFunction connects channel allocation sites with
// channel operations in the given function
// by using the results of the points-to analysis,
// and adds them to the receiver Goroutine.
func (g *Goro) ProcessFunction(fun *ssa.Function, results map[ssa.Value]pointer.Pointer) {
	localFun := isLocal(fun)
	addChanOp := func(i ssa.Instruction, v ssa.Value, chUpd func(*ChannelOp)) {
		labels := results[v].PointsTo().Labels()
		switch {
		case task.IsGoroTopology():
			for _, label := range labels {
				if isLocal(label.Value()) || localFun {
					chanOps := g.initOps(label.Value())
					chUpd(chanOps)
				}
			}
		case task.IsChannelAliasingCheck() && localFun:
			ChAliasingInfo.update(labels, i)
		}
	}

	for _, block := range fun.Blocks {
		for _, insn := range block.Instrs {
			switch i := insn.(type) {
			case *ssa.Call:
				switch callee := i.Call.Value.(type) {
				case *ssa.Builtin:
					if callee.Name() == "close" {
						addChanOp(i, i.Call.Args[0], func(chanOps *ChannelOp) {
							chanOps.Close = append(chanOps.Close, i)
						})
					}
				}
			case *ssa.MakeChan:
				if isLocal(i) || localFun {
					chanOps := g.initOps(i)
					chanOps.Buffer = i.Size
					chanOps.Make = i
				}
			case *ssa.UnOp:
				if i.Op == token.ARROW {
					addChanOp(i, i.X, func(co *ChannelOp) {
						co.Receive = append(co.Receive, i)
					})
				}
			case *ssa.Send:
				addChanOp(i, i.Chan, func(co *ChannelOp) {
					co.Send = append(co.Send, i)
				})
			case *ssa.Select:
				for _, state := range i.States {
					labels := results[state.Chan].PointsTo().Labels()
					switch {
					case task.IsGoroTopology():
						for _, label := range labels {
							if isLocal(label.Value()) || localFun {
								switch state.Dir {
								case types.SendOnly:
									chanOps := g.initOps(label.Value())
									chanOps.Send = append(chanOps.Send, i)
								case types.RecvOnly:
									chanOps := g.initOps(label.Value())
									chanOps.Receive = append(chanOps.Receive, i)
								}
							}
						}
					case task.IsChannelAliasingCheck() && localFun:
						ChAliasingInfo.update(labels, i)
					}
				}
			}
		}
	}
}

// Status: mildly tested
func CollectGoros(result *pointer.Result) (goros GoTopology) {
	cg := result.CallGraph
	// NOTE (O): I removed this call because it creates a disparity
	// between functions that are reachable via. the callgraph and
	// functions that are reachable in the static analysis.
	// If it is necessary to remove synthetic wrappers to construct the
	// goroutine topology we should find a way to copy the callgraph
	// beforehand.
	//cg.DeleteSyntheticNodes()

	visited := make(map[*ssa.Function]*Goro)
	queue := []*ssa.Function{}

	// Add a Callee of a go call edge to the queue if not visited
	discoverGoro := func(f *ssa.Function) *Goro {
		if _, found := visited[f]; !found {
			visited[f] = &Goro{
				Entry:             f,
				ChannelOperations: make(map[ssa.Value]*ChannelOp),
			}
			queue = append(queue, f)
		}

		return visited[f]
	}

	// TODO: Maybe revert to "discoverGoro(root)"
	root := cg.Root.Func
	for _, edge := range cg.Nodes[root].Out {
		discoverGoro(edge.Callee.Func)
	}

	popBack := func(queue *[]*ssa.Function) *ssa.Function {
		fun := (*queue)[len(*queue)-1]
		(*queue) = (*queue)[:len(*queue)-1]
		return fun
	}

	// While there are more goroutines to process
	for len(queue) > 0 {
		// Pick one goroutine
		fun := popBack(&queue)

		goro := visited[fun]
		goros = append(goros, goro)

		closure := make(map[*ssa.Function]bool)
		closure[fun] = true
		Q := []*ssa.Function{fun}

		// While there are unprocessed functions in the transitive closure
		for len(Q) > 0 {
			// Pick one function and process it
			cur := popBack(&Q)
			if !opts.JustGoros() {
				goro.ProcessFunction(cur, result.Queries)
			}

			// Check if the transitive closure should be expanded
			// or if there are any unseen goroutine targets
			for _, edge := range cg.Nodes[cur].Out {
				tar := edge.Callee.Func
				src := edge.Caller.Func
				allowed := false

				switch {
				case opts.LocalPackages():
					if src != nil && src.Pkg != nil {
						allowed = isLocal(src)
					}
					if tar != nil && tar.Pkg != nil {
						allowed = allowed || isLocal(tar)
					}
				case !opts.IncludeInternal():
					allowed = !pkgutil.CheckInGoroot(src) || !pkgutil.CheckInGoroot(tar)
				default:
					allowed = true
				}

				// Goroutine call or call that leaves package
				if _, isGo := edge.Site.(*ssa.Go); allowed && (opts.FullCg() || isGo || opts.PackageSplit() && tar.Pkg != fun.Pkg) {
					nextGoro := discoverGoro(tar)
					if isGo {
						goro.SpawnedGoroutines = append(goro.SpawnedGoroutines, nextGoro)
					} else {
						goro.CrossPackageCalls = append(goro.CrossPackageCalls, nextGoro)
					}
				} else if !closure[tar] {
					closure[tar] = true
					Q = append(Q, tar)
				}
			}
		}
	}

	return
}
