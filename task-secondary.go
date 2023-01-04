package main

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"

	"github.com/cs-au-dk/goat/analysis/gotopo"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	dotg "github.com/cs-au-dk/goat/graph"
	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/dot"
	"github.com/cs-au-dk/goat/utils/graph"
	"github.com/fatih/color"
	"golang.org/x/tools/go/ssa"
)

// secondaryTask checks whether a non-abstract interpretation focused task was provided,
// and executes it.
func (pl pipeline) secondaryTask() {
	switch {
	// static-metrics : computes the proportionality of numbers of callers/callees per call-site, and
	// points-to set size per channel operand.
	case task.IsStaticMetrics():
		pt, cfg := pl.preanalysisPipeline(u.IncludeType{All: true})

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
		orderedChanops, chOpsTotal := order(cfg.ChanOpsPointsToSets(&pt.Result))
		orderedChanImprecision, chTotal := order(cfg.CheckImpreciseChanOps(&pt.Result))

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

	// goroutine-topology : compute goroutine topology
	case task.IsGoroTopology():
		ptaResult, _, goros := pl.fullPreanalysisPipeline(standardPTAnalysisQueries)

		log.Println("Constructing topology graph...")
		image_path := dotg.BuildGraph(pl.prog, &ptaResult.Result, goros)
		fmt.Println(image_path)

	// cycle-check : checks that
	case task.IsCycleCheck():
		_, _, goros := pl.fullPreanalysisPipeline(standardPTAnalysisQueries)

		log.Println("Logging cycles in the goroutine topology graph...")
		goros.LogCycles()

	// points-to : prints the results of the points-to analysis
	case task.IsPointsTo():
		pt, _ := pl.preanalysisPipeline(u.IncludeType{All: true})

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
		pt, cfg := pl.preanalysisPipeline(u.IncludeType{All: true})

		G := graph.FromCallGraph(pt.CallGraph, true)
		psets := gotopo.GetInterprocPsets(cfg, pt, G)
		log.Println(psets)
	case task.IsWrittenFieldsAnalysis():
		// written-fields : prints the results of the written-field analysis.
		pt, _ := pl.preanalysisPipeline(u.IncludeType{All: true})
		cg := pt.CallGraph
		callDAG := graph.FromCallGraph(cg, true).SCC([]*ssa.Function{cg.Root.Func})

		wf := u.ComputeWrittenFields(pt, callDAG)

		log.Println(wf)

	case task.IsChannelAliasingCheck():
		pl.fullPreanalysisPipeline(standardPTAnalysisQueries)
		fmt.Printf("%d -- %s\n", u.ChAliasingInfo.MaxChanPtsToSetSize, u.ChAliasingInfo.Location)

	// callgraph-to-dot : visualizes the call graph.
	case task.IsCallGraphToDot():
		ptaResult, cfg := pl.preanalysisPipeline(standardPTAnalysisQueries)

		log.Println("Preparing to visualize callgraph:")
		cg := graph.FromCallGraph(ptaResult.CallGraph, false)
		root := ptaResult.CallGraph.Root.Func
		if opts.Function() != "main" {
			root = cfg.FunctionByName(opts.Function())
		}
		scc := cg.SCC([]*ssa.Function{root})
		allNodes := []*ssa.Function{}
		allComps := []int{}
		for i, comp := range scc.Components {
			anyLocal := false
			for _, node := range comp {
				if pkgutil.IsLocal(node) {
					anyLocal = true
					break
				}
			}
			if anyLocal || !opts.LocalPackages() {
				allNodes = append(allNodes, comp...)
				allComps = append(allComps, i)
			}
		}
		scc.Convolution().ToDotGraph(allComps, &graph.VisualizationConfig[int]{
			NodeAttrs: func(node int) (string, dot.DotAttrs) {
				return fmt.Sprint(node), dot.DotAttrs{"label": fmt.Sprint(scc.Components[node][0])}
			},
		}).ShowDot()
		cg.ToDotGraph(allNodes, &graph.VisualizationConfig[*ssa.Function]{
			ClusterKey: func(node *ssa.Function) any { return scc.ComponentOf(node) },
		}).ShowDot()

	// cfg-to-dot : visualizes the CFG.
	case task.IsCfgToDot():
		ptaResult, cfg := pl.preanalysisPipeline(standardPTAnalysisQueries)

		log.Println("Preparing to visualize CFG:")
		if opts.IsWholeProgramAnalysis() {
			cfg.Visualize(&ptaResult.Result)
		} else {
			cfg.VisualizeFunc(opts.Function())
		}

	// positions : prints the positions of all SSA functions.
	case task.IsPosition():
		for _, pkg := range pl.prog.AllPackages() {
			for _, member := range pkg.Members {
				switch f := member.(type) {
				case *ssa.Function:
					utils.PrintSSAFunWithPos(pl.prog.Fset, f)
				}
			}
		}
	}
}
