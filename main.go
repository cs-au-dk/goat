package main

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cs-au-dk/goat/analysis/upfront/chreflect"
	"github.com/cs-au-dk/goat/analysis/upfront/condinline"
	"github.com/cs-au-dk/goat/analysis/upfront/loopinline"
	"github.com/cs-au-dk/goat/pkgutil"
	tu "github.com/cs-au-dk/goat/testutil"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/graph"
	"github.com/cs-au-dk/goat/utils/hmap"
	"github.com/cs-au-dk/goat/vistool"

	ai "github.com/cs-au-dk/goat/analysis/absint"
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/analysis/gotopo"
	u "github.com/cs-au-dk/goat/analysis/upfront"

	"github.com/fatih/color"
	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"net/http"
	_ "net/http/pprof"
)

var (
	// opts exposes all the CLI passed configuration flags.
	opts = utils.Opts()
	// task exposes what task Goat is supposed to perform during the current execution
	task = opts.Task()
)

func main() {
	// Parse CLI provided arguments.
	utils.ParseArgs()
	// Retrieve target package path.
	path := utils.MakePath()

	// When debugging, set up pprof webview server.
	if opts.HttpDebug() {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	// Load packages at the given path, where LoadConfig extracts GOPATH,
	// module-aware settings and test inclusion from CLI flags.
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

	// Inline Cond variable constructors at AST level to improve points analysis precision.
	err = u.ASTTranform(pkgs, condinline.Transform)
	if err != nil {
		log.Fatalln("sync.NewCond inlining failed?", err)
	}

	// Set loops at one iteration at AST level.
	err = u.ASTTranform(pkgs, loopinline.Transform)
	if err != nil {
		log.Fatalln("Loop inlining failed?", err)
	}

	// If the task was to check whether the package could be build, stop here.
	if opts.Task().IsCanBuild() {
		return
	}

	// Convert program to SSA form and build it.
	prog, _ := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	// Extract all main package candidates from the SSA program.
	mains := ssautil.MainPackages(prog.AllPackages())
	if len(mains) == 0 {
		log.Println("No main packages detected")
		return
	}

	// Get all local packages in the SSA program.
	allPackages := pkgutil.AllPackages(prog)
	pkgutil.GetLocalPackages(mains, allPackages)

	// Map channel allocation sites to names, for the given
	if !opts.SkipChanNames() {
		u.CollectNames(pkgs)
	}

	aiConfig := ai.AIConfig{
		Metrics: opts.Metrics(),
		Log:     opts.LogAI(),
	}

	pl := pipeline{
		prog:  prog,
		mains: mains,
	}

	switch {
	default:
		// Execute secondary, non-abstract interpretation related tasks
		pl.secondaryTask()

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
		}
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
						// TODO: If we also want to analyse other concurrency primitives
						// we should use utils.AllocatesConcurrencyPrimitive instead here.
						if _, isMakeChan := insn.(*ssa.MakeChan); isMakeChan {
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

		var skips, aborts, completes int
		// Compute the pre-analysis
		pt, pcfg := pl.preanalysisPipeline(u.IncludeType{All: true})

		// Retrieve all the functions in the CFG
		cfgFunctions := pcfg.Functions()
		// Compute the sound and pruned call-graphs
		soundG := graph.FromCallGraph(pt.CallGraph, false)
		G := graph.FromCallGraph(pt.CallGraph, true)

		wf := u.ComputeWrittenFields(pt, G.SCC(entries))

		// Perform deduplication of fragments over all entry points.
		// This breaks joined use of flags -fun and -pset since the number
		// of psets for a function depends on analysis of previous functions.
		// (At least it's a little bit less obvious how to use them)
		allSeenFragments := map[*ssa.Function]*hmap.Map[utils.SSAValueSet, bool]{}

		// For every possible entry point
		for idx, entry := range entries {
			// If the analysis is targetted at a certain function, skip all functions
			// that do not match the query.
			if !opts.IsWholeProgramAnalysis() && !(strings.HasSuffix(entry.Name(), opts.Function()) ||
				strings.HasSuffix(entry.String(), opts.Function())) {
				continue
			}

			log.Printf("Entry %d of %d: %v", idx+1, len(entries), entry)

			// Compute call DAG
			callDAG := G.SCC([]*ssa.Function{entry})
			// Compute dominator tree as a variadic function over functions that
			// returns the dominator function.
			computeDominator := G.DominatorTree(entry)

			// Get concurrency primitives, and a map from concurrency primitives to
			// functions that use it.
			ps, primsToUses := gotopo.GetPrimitives(entry, pt, G, false)

			// Compute P-Sets based on the chosen P-Set strategy.
			psets := func() (psets gotopo.PSets) {
				switch {
				case opts.PSets().SameFunc():
					// Same function P-sets.
					return gotopo.GetSameFuncPsets(ps)
				case opts.PSets().GCatch():
					// GCatch Psets
					return gotopo.GetGCatchPSets(
						pcfg, entry, pt, G, computeDominator, callDAG, primsToUses)
				case opts.PSets().SCCS():
					// Inter-procedural dependencies
					return gotopo.GetSCCPSets(callDAG, primsToUses, pt)
				case opts.PSets().Total():
					// Singular whole program P-set
					return gotopo.GetTotalPset(ps)
				case opts.PSets().Singleton():
					// Singleton P-Sets
					fallthrough
				default:
					// Fallback to singleton P-sets as default
					return gotopo.GetSingletonPsets(ps)
				}
			}()

			// Remove channels that flow into the reflection library from P-sets.
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

			// If no P-sets remain, skip analysis.
			if len(psets) == 0 {
				log.Printf("%d primitives outside GOROOT reachable from %s", len(psets), entry)
				continue
			}

			// Ensure consistent ordering between P-sets.
			sort.Slice(psets, func(i, j int) bool {
				return psets[i].String() < psets[j].String()
			})

			// Define the type of fragments, pairing an entry function with a P-set.
			type fragment struct {
				entry *ssa.Function
				pset  utils.SSAValueSet
			}

			// Start from the empty set of fragments.
			fragments := []fragment{}

			// Compute analysis of P-set.
			for _, pset := range psets {
				// TODO: Protect dominator computation with flag?
				funs := []*ssa.Function{}

				// Aggregate a set of functions that includes all the channel operations
				// and the channel's allocation site.
				pset.ForEach(func(v ssa.Value) {
					if v.Parent() != nil {
						// Include allocation site in dominator computation
						funs = append(funs, v.Parent())
					}
					for fun := range primsToUses[v] {
						funs = append(funs, fun)
					}
				})

				// compute the dominator function that covers operations of all channels in the P-set.
				loweredEntry := computeDominator(funs...)
				if _, found := cfgFunctions[loweredEntry]; !found {
					log.Println(color.YellowString("CFG does not contain the entry function."))
					continue
				}

				// De-duplicate fragments based on entry point and P-sets.
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

			// Load options.
			loadRes := tu.LoadResult{
				// Program, main packages and CFG
				Prog:  prog,
				Mains: mains,
				Cfg:   pcfg,

				// Points-to analysis
				Pointer: pt,
				// Compute call DAG SCC with from the entry function
				CallDAG: soundG.SCC([]*ssa.Function{entry}),
				// Re-use pruned call DAG.
				PrunedCallDAG: callDAG,

				// Compute control location priorities and written-fields analysis
				CtrLocPriorities: u.GetCtrLocPriorities(cfgFunctions, callDAG),
				WrittenFields:    wf,
			}

			// For every fragment, run the analysis
			for i, fragment := range fragments {
				// If a specific P-set is targeted (by index), skip
				// until the desired index is found.
				if !opts.IsPickedPset(i + 1) {
					continue
				}

				// Extract dominator function as entry point and P-set from the fragment.
				loweredEntry, pset := fragment.entry, fragment.pset

				fmt.Println()
				log.Println(color.CyanString("Found PSet"), i+1, color.CyanString("of"), len(fragments), color.CyanString(":"))
				fmt.Println(pset)
				fmt.Println()

				log.Println("Using", loweredEntry, "as entrypoint")

				// Get analysis context and compute the fragment predicate from the given primitives.
				C := aiConfig.Function(loweredEntry)(loadRes)
				C.FragmentPredicateFromPrimitives(pset.Entries(), primsToUses)

				// Wire timeout function.
				done := make(chan bool, 1)
				timeout := 60 * time.Second
				go func() {
					select {
					case <-time.After(timeout):
						log.Println("Skipping")
						C.Skip()
					case <-done:
					}
				}()

				// Start static analysis and timer
				C.TimerStart()
				ts, analysis := ai.StaticAnalysis(C)
				done <- true

				log.Println("Superlocation graph size:", ts.Size())

				// Inspect outcome of static analysis
				switch C.Outcome {
				case ai.OUTCOME_SKIP:
					// Analysis skipped
					log.Println(color.RedString("Skipped!"))
					skips++
				case ai.OUTCOME_PANIC:
					// Analysis aborted
					log.Println(color.RedString("Aborted!"))
					log.Println(C.Error())
					aborts++
				default:
					// Analysis completed successfully.
					C.Done()
					log.Println(color.GreenString("SA completed in %s", C.Performance()))
					completes++

					// Compute blocking abstract thread analysis
					blocks := ai.BlockAnalysisFiltered(C, ts, analysis, true)

					if len(blocks) == 0 {
						log.Println(color.GreenString("No blocking bugs detected"))
					} else {
						// Log all the blocking bugs found.
						blocks.Log()

						if opts.HttpVisualize() {
							// Start HTTP visualization if HTTP visualization is an option.
							vistool.Start(C, ts, analysis, blocks)
						}

						if opts.Visualize() {
							// Visualize textual trace.
							blocks.PrintPath(ts, analysis, G)
							// Visualize blocking location.
							ts.Visualize(blocks)
						}
					}
				}
			}
		}

		log.Printf("Completed runs: %d, skipped runs: %d, aborted runs: %d", completes, skips, aborts)

	case task.IsAbstractInterpretation():
		ptaResult, prog_cfg := pl.preanalysisPipeline(u.IncludeType{All: true})

		results := make(map[*ssa.Function]*ai.Metrics)

		cg := ptaResult.CallGraph
		entries := []*ssa.Function{cg.Root.Func}
		loadRes := tu.LoadResult{
			Prog:    prog,
			Mains:   mains,
			Cfg:     prog_cfg,
			Pointer: ptaResult,
			CallDAG: graph.FromCallGraph(cg, false).SCC(entries),
		}
		loadRes.PrunedCallDAG = graph.FromCallGraph(cg, true).SCC(entries)
		loadRes.CtrLocPriorities = u.GetCtrLocPriorities(prog_cfg.Functions(), loadRes.PrunedCallDAG)
		loadRes.WrittenFields = u.ComputeWrittenFields(ptaResult, loadRes.PrunedCallDAG)

		// Analysis context
		Cs := aiConfig.Executable(loadRes)

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
					G.Visualize(blocks)
				}

				continue
			}

			done := make(chan bool)
			closed := make(chan bool)
			mu := &sync.Mutex{}

			// Allocate space for spawned goroutines
			go func(f *ssa.Function, C ai.AnalysisCtxt) {
				var blocks ai.Blocks
				C.TimerStart()
				defer func() {
					if C.HasConcurrency() {
						// ops := C.ConcurrencyOps
						// fs := C.ExpandedFunctions
						mu.Lock()
						C.Done()
						results[f] = C.Metrics
						mu.Unlock()
					}
				}()
				defer func() {
					if err := recover(); err != nil {
						mu.Lock()
						if _, ok := results[f]; !ok {
							C.Panic(err)
							results[f] = C.Metrics
						}
						mu.Unlock()
						close(done)
						return
					}

					mu.Lock()
					if _, ok := results[f]; !ok {
						C.Done()
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
				if C.IsRelevant() {
					C.SetBlocks(blocks)
				}
				if !C.Metrics.Enabled() {
					blocks.Log()
					if opts.Visualize() {
						G.Visualize(blocks)
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
						C.Skip()
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

		gatherMetrics(loadRes, results)
	}
}
