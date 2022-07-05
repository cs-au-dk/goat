package main

import (
	// "Goat/solver"

	"fmt"
	"go/types"
	"log"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"Goat/analysis/upfront/chreflect"
	"Goat/analysis/upfront/loopinline"
	dot "Goat/graph"
	"Goat/pkgutil"
	tu "Goat/testutil"
	"Goat/utils"
	"Goat/utils/graph"
	"Goat/utils/hmap"

	ai "Goat/analysis/absint"
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	"Goat/analysis/gotopo"
	u "Goat/analysis/upfront"

	"github.com/fatih/color"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"net/http"
	_ "net/http/pprof"
)

var (
	opts = utils.Opts()
	task = opts.Task()
)

func main() {
	utils.ParseArgs()
	path := utils.MakePath()

	if opts.HttpDebug() {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	pkgs, err := pkgutil.LoadPackages(pkgutil.LoadConfig{
		GoPath:       opts.GoPath(),
		ModulePath:   opts.ModulePath(),
		IncludeTests: opts.IncludeTests(),
	}, path)
	if err != nil {
		log.Println("Failed pkgutil.LoadPackages")
		log.Println(err)
		os.Exit(1)
	}

	// pkgs = u.UnrollLoops(pkgs)
	err = loopinline.InlineLoops(pkgs)
	if err != nil {
		log.Fatalln("Loop inlining failed?", err)
	}

	if opts.Task().IsCanBuild() {
		return
	}

	prog, _ := ssautil.AllPackages(pkgs, 0)
	prog.Build()

	mains := ssautil.MainPackages(prog.AllPackages())

	if len(mains) == 0 {
		log.Println("No main packages detected")
		return
	}

	allPackages := pkgutil.AllPackages(prog)
	pkgutil.GetLocalPackages(mains, allPackages)

	if !opts.SkipChanNames() {
		u.CollectNames(pkgs)
	}

	// Assemble pre-analysis preanalysisPipeline
	preanalysisPipeline := func(includes u.IncludeType) (*pointer.Result, *cfg.Cfg) {
		fmt.Println()
		log.Println("Performing points-to analysis...")
		ptaResult := u.Andersen(prog, mains, includes)
		log.Println("Points-to analysis done")
		fmt.Println()

		log.Println("Extending CFG...")
		progCfg := cfg.GetCFG(prog, mains, ptaResult)
		log.Println("CFG extensions done")
		fmt.Println()

		opts.OnVerbose(func() {
			for val, ptr := range ptaResult.Queries {
				fmt.Printf("Points to information for \"%s\" at %d (%s):\n",
					val, val.Pos(), prog.Fset.Position(val.Pos()))
				for _, label := range ptr.PointsTo().Labels() {
					fmt.Printf("%s : %d (%s), ", label, (*label).Pos(), prog.Fset.Position((*label).Pos()))
				}
				fmt.Print("\n\n")
			}
		})

		return ptaResult, progCfg
	}

	fullPreanalysisPipeline := func(includes u.IncludeType) (
		*pointer.Result,
		*cfg.Cfg,
		u.GoTopology,
	) {
		ptaResult, progCfg := preanalysisPipeline(includes)

		log.Println("Constructing Goroutine topology...")
		goros := u.CollectGoros(ptaResult)
		log.Println("Goroutine topology done")

		opts.OnVerbose(func() {
			fmt.Println("Found the following goroutines:")
			for _, goro := range goros {
				fmt.Println(goro.String())
				fmt.Println()
			}
			fmt.Println()
		})

		return ptaResult, progCfg, goros
	}

	// States queries for which types to include the Andersen points-to analysis
	standardPTAnalysisQueries := u.IncludeType{
		Chan:      true,
		Function:  true,
		Interface: true,
	}

	aiConfig := ai.AIConfig{
		Metrics: opts.Metrics(),
		Log:     opts.LogAI(),
	}

	switch {
	case task.IsStaticMetrics():
		pt, cfg := preanalysisPipeline(u.IncludeType{All: true})

		cs, callees := cfg.MaxCallees()

		prec2 := func(n float64) float64 {
			return math.Floor(n*100) / 100
		}

		order := func(count map[int]int) (ordered []struct{ count, nodes int }, total int) {
			for c, nodes := range count {
				ordered = append(ordered, struct {
					count, nodes int
				}{c, nodes})
				total += nodes
			}
			sort.Slice(ordered, func(i, j int) bool {
				return ordered[i].count < ordered[j].count
			})

			return
		}

		fmt.Println("================ Results =====================")
		fmt.Println("Maximum callees for a call-site:", color.BlueString(cs.String()), color.GreenString(strconv.Itoa(callees)))

		orderedCallsites, callsiteTotal := order(cfg.CalleeCount())
		orderedExitnodes, exitsTotal := order(cfg.CallerCount())
		orderedChanops, chOpsTotal := order(cfg.ChanOpsPointsToSets(pt))
		orderedChanImprecision, chTotal := order(cfg.CheckImpreciseChanOps(pt))

		type printConfig = struct {
			source string
			drain  string
			total  int
		}

		print := func(conf printConfig, orderedSites []struct{ count, nodes int }) {
			for _, o := range orderedSites {
				c, nodes := o.count, o.nodes

				percent := prec2(float64(nodes) / float64(conf.total) * 100)
				var colorize func(string, ...interface{}) string
				switch {
				case c <= 1:
					colorize = color.BlueString
				case c == 2:
					colorize = color.GreenString
				case 3 <= c && c <= 5:
					colorize = color.YellowString
				default:
					colorize = color.HiRedString
				}

				fmt.Println(colorize("%v", c), " "+conf.drain+" found at", percent, "% ("+color.HiCyanString("%v", nodes)+") of", conf.source)
			}
		}

		fmt.Println("\nOutgoing degree for function call/method invocation sites")
		print(printConfig{
			source: "call sites",
			drain:  "callees",
			total:  callsiteTotal,
		}, orderedCallsites)

		fmt.Println("\nOutgoing degree for function exit nodes")
		print(printConfig{
			source: "function exit nodes",
			drain:  "callers",
			total:  exitsTotal,
		}, orderedExitnodes)

		fmt.Println("\nPoints-to set cardinality for channel operands of channel operations")
		print(printConfig{
			source: "channel operations",
			drain:  "channel operands in points-to set",
			total:  chOpsTotal,
		}, orderedChanops)

		fmt.Println("\nChannel primitive imprecision")
		print(printConfig{
			source: "channels",
			drain:  "maximum channels which may alias at the same operation",
			total:  chTotal,
		}, orderedChanImprecision)

	case task.IsGoroTopology():
		ptaResult, _, goros := fullPreanalysisPipeline(standardPTAnalysisQueries)

		log.Println("Constructing topology graph...")
		image_path := dot.BuildGraph(prog, ptaResult, goros)
		fmt.Println(image_path)
	case task.IsCycleCheck():
		_, _, goros := fullPreanalysisPipeline(standardPTAnalysisQueries)

		log.Println("Logging cycles in the goroutine topology graph...")
		goros.LogCycles()
	case task.IsPointsTo():
		pt, _ := preanalysisPipeline(u.IncludeType{All: true})

		if len(pt.Warnings) > 0 {
			fmt.Println("Warnings:")
			for _, w := range pt.Warnings {
				fmt.Println(w)
			}
		}

		fmt.Println()
		log.Println("Points-to analysis results:")
		fmt.Println("Direct queries:")
		for v, ptset := range pt.Queries {
			if opts.LocalPackages() && !pkgutil.IsLocal(v) {
				continue
			}
			fmt.Println("SSA Value", utils.SSAValString(v))
			fmt.Println("Points to: {")
			str := ""
			for _, l := range ptset.PointsTo().Labels() {
				lv := l.Value()
				str += "\t" + utils.SSAValString(lv) + ",\n"
			}
			str += "}"
			fmt.Println(str)
		}
		fmt.Println("")
		fmt.Println("Indirect queries:")
		for v, ptset := range pt.IndirectQueries {
			if opts.LocalPackages() && !pkgutil.IsLocal(v) {
				continue
			}
			fmt.Println("SSA Value", utils.SSAValString(v))
			fmt.Println("Indirectly points to: {")
			str := ""
			for _, l := range ptset.PointsTo().Labels() {
				lv := l.Value()
				str += "\t" + utils.SSAValString(lv) + ",\n"
			}
			str += "}"
			fmt.Println(str)
		}
	case task.IsCheckPsets():
		pt, cfg := preanalysisPipeline(u.IncludeType{All: true})

		G := graph.FromCallGraph(pt.CallGraph, true)
		psets := gotopo.GetInterprocPsets(cfg, pt, G)
		log.Println(psets)
	case task.IsWrittenFieldsAnalysis():
		pt, _ := preanalysisPipeline(u.IncludeType{All: true})
		cg := pt.CallGraph
		callDAG := graph.FromCallGraph(cg, true).SCC([]*ssa.Function{cg.Root.Func})

		wf := u.ComputeWrittenFields(pt, callDAG)

		log.Println(wf)
	case task.IsCollectPrimitives():
		if !opts.Metrics() {
			log.Fatalln("Run with -metrics")
		}

		entries := pkgutil.TestFunctions(prog)

		for _, main := range ssautil.MainPackages(allPackages) {
			entries = append(entries, main.Func("main"))
		}

		sort.Slice(entries, func(i, j int) bool {
			return entries[i].String() < entries[j].String()
		})

		if len(entries) == 0 {
			log.Println("Skipping benchmark since it has no entries")
			return
		} else {
			// This is a cheap check to see if we can avoid processing the package altogether.
			// If there is no reachable local channel allocation in the RTA call graph,
			// there is no need to do expensive (pointer) pre-analyses.
			log.Println("Building initial RTA callgraph")
			rtaCG := rta.Analyze(entries, true).CallGraph
			rtaG := graph.FromCallGraph(rtaCG, false)
			log.Printf("RTA callgraph constructed with %d nodes", len(rtaCG.Nodes))

			// Check if an entry can reach a local channel allocation in the RTA call graph
			if !rtaG.BFSV(func(fun *ssa.Function) bool {
				if pkgutil.IsLocal(fun) {
					for _, block := range fun.Blocks {
						for _, insn := range block.Instrs {
							if _, isMakeChan := insn.(*ssa.MakeChan); isMakeChan {
								//log.Println(insn, prog.Fset.Position(insn.Pos()))
								return true
							}
						}
					}
				}
				return false
			}, entries...) {
				log.Println("Skipping benchmark since it has no local channel allocations")
				return
			}
		}

		skips, aborts, completes := 0, 0, 0
		pt, pcfg := preanalysisPipeline(u.IncludeType{All: true})
		cfgFunctions := pcfg.Functions()
		G := graph.FromCallGraph(pt.CallGraph, true)
		wf := u.ComputeWrittenFields(pt, G.SCC(entries))

		// Perform deduplication of fragments over all entry points.
		// This breaks joined use of flags -fun and -pset since the number
		// of psets for a function depends on analysis of previous functions.
		// (At least it's a little bit less obvious how to use them)
		allSeenFragments := map[*ssa.Function]*hmap.Map[utils.SSAValueSet, bool]{}

		for idx, entry := range entries {
			if !opts.IsWholeProgramAnalysis() && !(strings.HasSuffix(entry.Name(), opts.Function()) ||
				strings.HasSuffix(entry.String(), opts.Function())) {
				continue
			}

			log.Printf("Entry %d of %d: %v", idx+1, len(entries), entry)

			/*
				var spkg *ssa.Package
				if entry.Name() == "main" {
					spkg = entry.Package()
				} else {
					spkg = pkgutil.CreateFakeTestMainPackage(entry)
				}

				mains = []*ssa.Package{spkg}
			*/

			callDAG := G.SCC([]*ssa.Function{entry})
			computeDominator := G.DominatorTree(entry)

			ps := gotopo.GetPrimitives(entry, pt, G)
			// Filter out non-local primitives and primitives whose allocation
			// site are not reachable in the call graph.
			// Currently also filters out non-channel primitives.
			for _, usageInfo := range ps {
				for _, usedPrimitives := range []map[ssa.Value]struct{}{
					usageInfo.Chans(),
					usageInfo.OutChans(),
					// usageInfo.Sync(),
				} {
					for prim := range usedPrimitives {
						if _, isCh := prim.Type().Underlying().(*types.Chan); !isCh ||
							prim.Parent() == nil || !pkgutil.IsLocal(prim) ||
							callDAG.ComponentOf(prim.Parent()) == -1 {
							delete(usedPrimitives, prim)
						}
					}
				}
			}

			psets := func() (psets gotopo.PSets) {
				switch {
				case opts.PSets().SameFunc():
					return gotopo.GetSameFuncPsets(ps)
				case opts.PSets().GCatch():
					return gotopo.GetGCatchPSets(
						pcfg, entry, pt, G,
						computeDominator, callDAG, ps) // GCatch Psets
				case opts.PSets().Total():
					return gotopo.GetTotalPset(ps) // Singular whole program p-set
				case opts.PSets().Singleton():
					fallthrough
				default:
					return gotopo.GetSingletonPsets(ps) // Singleton sets
				}
			}()

			// Remove channels from PSets where the channel flows into the reflection library
			if reflectedChans := chreflect.GetReflectedChannels(prog, pt); !reflectedChans.Empty() {
				log.Printf("%s:\n%v",
					color.YellowString("Pruning channels from PSets that flow into the reflection library"),
					reflectedChans)

				// Since pruning may introduce duplicate PSets, we use hashing
				// to ensure they are not present in the output.
				newPsets := make(gotopo.PSets, 0, len(psets))
				seen := hmap.NewMap[bool](utils.SSAValueSetHasher)

				for _, pset := range psets {
					reflectedChans.ForEach(func(rCh ssa.Value) {
						pset.Map = pset.Delete(rCh)
					})

					if !pset.Empty() && !seen.Get(pset) {
						seen.Set(pset, true)
						newPsets = append(newPsets, pset)
					}
				}

				psets = newPsets
			}

			//log.Printf("%s", psets)

			if len(psets) == 0 {
				log.Printf("%d primitives outside GOROOT reachable from %s", len(psets), entry)
				continue
			}

			// Ensure consistent ordering
			sort.Slice(psets, func(i, j int) bool {
				return psets[i].String() < psets[j].String()
			})

			// Compute map from primitives to functions in which they are used
			primsToUses := map[ssa.Value]map[*ssa.Function]struct{}{}
			for fun, usageInfo := range ps {
				if callDAG.ComponentOf(fun) == -1 {
					continue
				}

				for _, usedPrimitives := range []map[ssa.Value]struct{}{
					usageInfo.Chans(),
					usageInfo.OutChans(),
					// TODO: Temporarily disabled because analysis of locks
					// seem to time out more often than analysis of channels.
					//usageInfo.Sync(),
				} {
					for prim := range usedPrimitives {
						if _, seen := primsToUses[prim]; !seen {
							primsToUses[prim] = make(map[*ssa.Function]struct{})
						}
						primsToUses[prim][fun] = struct{}{}
					}
				}
			}

			type fragment struct {
				entry *ssa.Function
				pset  utils.SSAValueSet
			}

			fragments := []fragment{}
			for _, pset := range psets {
				// TODO: Protect dominator computation with flag?
				funs := []*ssa.Function{}
				pset.ForEach(func(v ssa.Value) {
					if v.Parent() != nil {
						// Include allocation site in dominator computation
						funs = append(funs, v.Parent())
					}
					for fun := range primsToUses[v] {
						funs = append(funs, fun)
					}
				})

				loweredEntry := computeDominator(funs...)
				if _, found := cfgFunctions[loweredEntry]; !found {
					log.Println(color.YellowString("CFG does not contain the entry function."))
					continue
				}

				seenFragments, ok := allSeenFragments[loweredEntry]
				if !ok {
					mp := hmap.NewMap[bool](utils.SSAValueSetHasher)
					allSeenFragments[loweredEntry] = mp
					seenFragments = mp
				}

				if !seenFragments.Get(pset) {
					seenFragments.Set(pset, true)
					fragments = append(fragments, fragment{loweredEntry, pset})
				}
			}

			log.Printf("%d primitives outside GOROOT reachable from %s", len(fragments), entry)

			loadRes := tu.LoadResult{
				Prog:             prog,
				Mains:            mains,
				Cfg:              pcfg,
				Pointer:          pt,
				CallDAG:          callDAG,
				CtrLocPriorities: u.GetCtrLocPriorities(cfgFunctions, callDAG),
				WrittenFields:    wf,
			}

			for i, fragment := range fragments {
				if !opts.IsPickedPset(i + 1) {
					continue
				}

				loweredEntry, pset := fragment.entry, fragment.pset

				fmt.Println()
				log.Println(color.CyanString("Found PSet"), i+1, color.CyanString("of"), len(fragments), color.CyanString(":"))
				fmt.Println(pset)
				fmt.Println()

				log.Println("Using", loweredEntry, "as entrypoint")

				C := ai.ConfigAI(aiConfig).Function(loweredEntry)(loadRes)
				C.FragmentPredicateFromPrimitives(pset.Entries(), primsToUses)

				done := make(chan bool, 1)
				timeout := 60 * time.Second
				go func() {
					select {
					case <-time.After(timeout):
						log.Println("Skipping")
						C.Metrics.Skip()
					case <-done:
					}
				}()

				C.Metrics.TimerStart()
				ts, analysis := ai.StaticAnalysis(C)
				done <- true

				log.Println("Superlocation graph size:", ts.Size())

				switch C.Metrics.Outcome {
				case ai.OUTCOME_SKIP:
					log.Println(color.RedString("Skipped!"))
					skips++
				case ai.OUTCOME_PANIC:
					log.Println(color.RedString("Aborted!"))
					log.Println(C.Metrics.Error())
					aborts++
				default:
					C.Metrics.Done()
					log.Println(color.GreenString("SA completed in %s", C.Metrics.Performance()))
					completes++

					blocks := ai.BlockAnalysis(C, ts, analysis)
					if len(blocks) == 0 {
						log.Println(color.GreenString("No blocking bugs detected"))
					} else {
						blocks.Log()

						if opts.Visualize() {
							blocks.PrintPath(ts, analysis, pset)
							ts.Entry().Visualize()
						}
					}

				}

				/*
					allocSiteExpansions := 0
					pset.ForEach(func(prim ssa.Value) {
						allocSiteExpansions += C.Metrics.Functions()[prim.Parent()]
					})

					syncConfsWithPrimitive := 0
					ts.ForEach(func(conf *ai.AbsConfiguration) {
						state := analysis.GetUnsafe(conf.Superlocation())
						if !conf.IsPanicked() && conf.IsSynchronizing(C, state) {
							mem := state.Memory()
							_, _, found := conf.Threads().Find(func(g defs.Goro, cl defs.CtrLoc) bool {
								for _, prim := range cfg.CommunicationPrimitivesOf(cl.Node()) {
									if av := ai.EvaluateSSA(g, mem, prim); av.IsPointer() {
										for _, ptr := range av.PointerValue().Entries() {
											site, _ := ptr.GetSite()
											if C.IsPrimitiveFocused(site) {
												return true
											}
										}
									}
								}
								return false
							})

							if found {
								syncConfsWithPrimitive++
							}
						}
					})

					log.Printf("allocSiteExpansions: %d, synchronizing configurations with primitive: %d",
						allocSiteExpansions, syncConfsWithPrimitive)
				*/
			}
		}

		log.Printf("Completed runs: %d, skipped runs: %d, aborted runs: %d", completes, skips, aborts)

	case task.IsChannelAliasingCheck():
		fullPreanalysisPipeline(standardPTAnalysisQueries)
		fmt.Printf("%d -- %s\n", u.ChAliasingInfo.MaxChanPtsToSetSize, u.ChAliasingInfo.Location)
	case task.IsCfgToDot():
		ptaResult, _ := preanalysisPipeline(standardPTAnalysisQueries)

		log.Println("Preparing to visualize CFG:")
		if opts.IsWholeProgramAnalysis() {
			cfg.Visualize(prog, ptaResult)
		} else {
			cfg.VisualizeFunc(prog, ptaResult, opts.Function())
		}
	case task.IsAbstractInterpretation():
		ptQueries := u.IncludeType{All: true}
		if !task.IsWholeProgramAnalysis() {
			ptQueries = u.IncludeType{All: true}
		}

		results := make(map[*ssa.Function]*ai.Metrics)

		ptaResult, prog_cfg := preanalysisPipeline(ptQueries)
		loadRes := tu.LoadResult{
			Prog:    prog,
			Mains:   mains,
			Cfg:     prog_cfg,
			Pointer: ptaResult,
		}
		cg := ptaResult.CallGraph
		G := graph.FromCallGraph(cg, true)
		loadRes.CallDAG = G.SCC([]*ssa.Function{cg.Root.Func})
		loadRes.CtrLocPriorities = u.GetCtrLocPriorities(prog_cfg.Functions(), loadRes.CallDAG)
		loadRes.WrittenFields = u.ComputeWrittenFields(ptaResult, loadRes.CallDAG)

		// Analysis context
		Cs := ai.ConfigAI(aiConfig).Executable(loadRes)

		timeout := 120000 * time.Millisecond

		for f, C := range Cs {
			if !C.Metrics.Enabled() {
				log.Println("Abstractly interpreting:")
				fmt.Println(f)
				fmt.Println("Found at", prog.Fset.Position(f.Pos()))
				fmt.Println()
			}

			if !opts.AnalyzeAllFuncs() {
				var blocks ai.Blocks

				G, A := ai.StaticAnalysis(C)
				log.Println("Done")
				fmt.Println()
				log.Println("Analysis result:\n", A.ProjectMemory())
				// if G.Size() > 3 {
				// }
				// Log all the found blocking bugs.
				blocks = ai.BlockAnalysis(C, G, A)
				blocks.ForEach(func(sl defs.Superloc, gs map[defs.Goro]struct{}) {
					fmt.Printf("%s ↦ %s\n", sl, A.GetUnsafe(sl).Memory())
				})
				blocks.Log()
				if opts.Visualize() {
					G.Entry().Visualize()
				}

				continue
			}

			done := make(chan bool)
			closed := make(chan bool)
			mu := &sync.Mutex{}

			// Allocate space for spawned goroutines
			go func(f *ssa.Function, C ai.AnalysisCtxt) {
				var blocks ai.Blocks
				C.Metrics.TimerStart()
				defer func() {
					if C.Metrics.HasConcurrency() {
						// ops := C.ConcurrencyOps
						// fs := C.ExpandedFunctions
						mu.Lock()
						C.Metrics.Done()
						results[f] = C.Metrics
						mu.Unlock()
					}
				}()
				defer func() {
					if err := recover(); err != nil {
						mu.Lock()
						if _, ok := results[f]; !ok {
							C.Metrics.Panic(err)
							results[f] = C.Metrics
						}
						mu.Unlock()
						close(done)
						return
					}

					mu.Lock()
					if _, ok := results[f]; !ok {
						C.Metrics.Done()
						results[f] = C.Metrics
					}
					mu.Unlock()

					close(done)
				}()

				G, result := ai.StaticAnalysis(C)
				if !C.Metrics.Enabled() {
					log.Println("Done")
					fmt.Println()
				}
				blocks = ai.BlockAnalysis(C, G, result)
				// log.Println("Analysis result:\n", A)
				if C.Metrics.IsRelevant() {
					C.Metrics.SetBlocks(blocks)
				}
				if !C.Metrics.Enabled() {
					blocks.Log()
					if opts.Visualize() {
						G.Entry().Visualize()
					}
				}
			}(f, C)

			go func(f *ssa.Function, C ai.AnalysisCtxt) {
				select {
				case <-done:
				case <-time.After(timeout):
					if !C.Metrics.Enabled() {
						fmt.Println("Function", f, "takes too long to analyze. Skip?")
						utils.Prompt()
						fmt.Println("Function", f, "skipped")
					}
					close(closed)

					mu.Lock()
					if _, ok := results[f]; !ok {
						C.Metrics.Skip()
						results[f] = C.Metrics
					}
					mu.Unlock()
				}
			}(f, C)

			select {
			case <-done:
			case <-closed:
			}
		}

		GatherMetrics(loadRes, results)
	case task.IsPosition():
		for _, pkg := range prog.AllPackages() {
			for _, member := range pkg.Members {
				switch f := member.(type) {
				case *ssa.Function:
					utils.PrintSSAFunWithPos(prog.Fset, f)
				}
			}
		}
	}
}

func GatherMetrics(loadRes tu.LoadResult, results map[*ssa.Function]*ai.Metrics) {
	if !opts.Metrics() || len(results) == 0 {
		return
	}
	prog := loadRes.Prog
	coveredConcOp := make(map[ssa.Instruction]struct{})
	coveredChans := make(map[ssa.Instruction]struct{})
	coveredGos := make(map[ssa.Instruction]struct{})

	msg := "================ Results =====================\n\n"

	for f, r := range results {
		msg += "Function: " + f.String() + "\n"
		msg += "Outcome: " + r.Outcome + "\n"

		if r.Outcome == ai.OUTCOME_SKIP {
			msg += "Function finished\n\n"
			continue
		}
		if r.Outcome == ai.OUTCOME_PANIC {
			msg += r.Error() + "\nFunction finished\n\n"
			continue
		}

		msg += "Time: " + r.Performance() + "\n\n"

		files := make(map[string]struct{})

		if len(r.Functions()) > 0 {
			msg += "Expanded functions: " + fmt.Sprintf("%d", len(r.Functions())) + " {\n"
			for f, times := range r.Functions() {
				fn := prog.Fset.Position(f.Pos()).Filename
				if _, ok := files[fn]; !ok {
					files[prog.Fset.Position(f.Pos()).Filename] = struct{}{}
				}

				msg += "  " + f.String() + " -- " + fmt.Sprintf("%d", times) + "\n"
			}
			msg += "}\n"
		}

		if len(r.Blocks()) > 0 {
			msg += "Blocks:"
			msg += r.Blocks().String()
			msg += "\n"
		}

		fs := make([]string, 0, len(files))
		for f := range files {
			fs = append(fs, f)
		}
		if len(fs) > 0 {
			cloc := exec.Command("cloc", fs...)
			out, err := cloc.Output()
			if err == nil {
				msg += string(out) + "\n"
			}
		}

		for i := range r.ConcurrencyOps() {
			coveredConcOp[i] = struct{}{}
		}
		for i := range r.Gos() {
			coveredGos[i] = struct{}{}
		}
		for i := range r.Chans() {
			coveredChans[i] = struct{}{}
		}
		msg += "Function finished\n\n"
	}

	allConcOps := loadRes.Cfg.GetAllConcurrencyOps()
	msg += "Concurrency operations covered: " + fmt.Sprint(len(coveredConcOp)) + "/" + fmt.Sprint(len(allConcOps)) + " {\n"
	if len(allConcOps) > 0 {
		notCovered := make(map[ssa.Instruction]struct{})
		for op := range allConcOps {
			if _, ok := coveredConcOp[op]; !ok {
				notCovered[op] = struct{}{}
			}
		}
		if len(notCovered) > 0 {
			msg += "Not covered: {\n"
			for op := range notCovered {
				msg += "  " + op.String() + ":" + prog.Fset.Position(op.Pos()).String() + "\n"
			}
			msg += "}\n"
		}
	}

	allChans := loadRes.Cfg.GetAllChans()
	msg += "Channel sites covered: " + fmt.Sprint(len(coveredChans)) + "/" + fmt.Sprint(len(allChans)) + "\n"
	if len(allChans) > 0 {
		notCovered := make(map[ssa.Instruction]struct{})
		for ch := range allChans {
			if _, ok := coveredChans[ch]; !ok {
				notCovered[ch] = struct{}{}
			}
		}
		if len(notCovered) > 0 {
			msg += "Not covered: {\n"
			for ch := range notCovered {
				msg += "  " + ch.String() + ":" + prog.Fset.Position(ch.Pos()).String() + "\n"
			}
			msg += "}\n"
		}
	}

	allGos := loadRes.Cfg.GetAllGos()
	msg += "Goroutine sites covered: " + fmt.Sprint(len(coveredGos)) + "/" + fmt.Sprint(len(allGos)) + "\n"
	if len(allGos) > 0 {
		notCovered := make(map[ssa.Instruction]struct{})
		for g := range allGos {
			if _, ok := coveredGos[g]; !ok {
				notCovered[g] = struct{}{}
			}
		}
		if len(notCovered) > 0 {
			msg += "Not covered: {\n"
			for g := range notCovered {
				msg += "  " + g.String() + ":" + prog.Fset.Position(g.Pos()).String() + "\n"
			}
			msg += "}\n"
		}
	}
	msg += "================ Results ====================="
	fmt.Println(msg)
}
