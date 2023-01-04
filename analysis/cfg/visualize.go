package cfg

import (
	"fmt"
	"log"
	"strconv"

	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/dot"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// Visualize creates a Dot Graph representing the program CFG
func (cfg *Cfg) Visualize(result *pointer.Result) {
	G := &dot.DotGraph{
		Options: map[string]string{
			"minlen":  fmt.Sprint(opts.Minlen()),
			"nodesep": fmt.Sprint(opts.Nodesep()),
			"rankdir": "TB",
		},
	}
	fmt.Println()

	nodeToDotNode := make(map[Node]*dot.DotNode)

	// Assumes that from and to are present in nodeToDotNode
	addEdge := func(from, to Node, attrs dot.DotAttrs) {
		G.Edges = append(G.Edges, &dot.DotEdge{
			From:  nodeToDotNode[from],
			To:    nodeToDotNode[to],
			Attrs: attrs,
		})
	}

	// Build visualization only for reachable functions
	//for fun := range ssautil.AllFunctions(prog) {
	for fun := range result.CallGraph.Nodes {
		if opts.LocalPackages() && !pkgutil.IsLocal(fun) {
			continue
		}

		if !cfg.addFunctionToVisualizationGraph(G, nodeToDotNode, fun) {
			log.Printf("CFG for local function %q not present?", fun.Name())
			continue
		}
	}

	// Add interprocedural edges
	for fun := range result.CallGraph.Nodes {
		if opts.LocalPackages() && !pkgutil.IsLocal(fun) {
			continue
		}

		fentry, ok := cfg.funs[fun]
		if !ok {
			continue
		}
		nodesInFunc := fentry.nodes

		for node := range nodesInFunc {
			for next := range node.Successors() {
				// Only add edges for nodes that are not in the same function
				if _, ok := nodesInFunc[next]; !ok && nodeToDotNode[next] != nil {
					addEdge(node, next, dot.DotAttrs{
						"style": "bold",
					})
				}
			}
			for next := range node.Spawns() {
				// Only add edges for nodes that are not in the same function
				if _, ok := nodesInFunc[next]; !ok && nodeToDotNode[next] != nil {
					addEdge(node, next, dot.DotAttrs{
						"style": "bold, dashed",
					})
				}
			}
		}
	}

	G.ShowDot()
	/*
		var buf bytes.Buffer
		if err := G.WriteDot(&buf); err != nil {
			log.Fatal(err)
		}

		// Inspect DOT output?
		// log.Println(buf.String())

		out, err := dot.DotToImage("", utils.Opts.OutputFormat, buf.Bytes())
		if err != nil {
			log.Fatal(err)
		}

		log.Println(out)
	*/
}

/* Creates a Dot Graph representing the program CFG */
func (cfg *Cfg) VisualizeFunc(fun string) {
	f := cfg.FunctionByName(fun)
	cfg.VisualizeFunction(f)
}
func (cfg *Cfg) VisualizeFunction(fun *ssa.Function) {
	G := &dot.DotGraph{
		Options: map[string]string{
			"minlen":  fmt.Sprint(opts.Minlen()),
			"nodesep": fmt.Sprint(opts.Nodesep()),
			"rankdir": "TB",
		},
	}
	fmt.Println()

	nodeToDotNode := make(map[Node]*dot.DotNode)

	if !cfg.addFunctionToVisualizationGraph(G, nodeToDotNode, fun) {
		panic(fmt.Sprintf("CFG for %s does not exist?", fun))
	}

	G.ShowDot()
}

func (cfg *Cfg) addFunctionToVisualizationGraph(G *dot.DotGraph, nodeToDotNode map[Node]*dot.DotNode, fun *ssa.Function) bool {
	// Assumes that from and to are present in nodeToDotNode
	addEdge := func(from, to Node, attrs dot.DotAttrs) {
		G.Edges = append(G.Edges, &dot.DotEdge{
			From:  nodeToDotNode[from],
			To:    nodeToDotNode[to],
			Attrs: attrs,
		})
	}

	fentry, ok := cfg.funs[fun]
	if !ok {
		return false
	}
	nodesInFunc := fentry.nodes

	funId := utils.SSAFunString(fun)
	cluster := dot.NewDotCluster(funId)
	cluster.Attrs["label"] = funId
	cluster.Attrs["bgcolor"] = "#e6ffff"

	G.Clusters = append(G.Clusters, cluster)

	// Add nodes
	for node := range nodesInFunc {
		var nodeId string
		var block *ssa.BasicBlock
		switch n := node.(type) {
		case *SSANode:
			if n.insn.Block() != nil {
				block = n.insn.Block()
			}
			// Use the pointer value of the ssa.Instruction to uniquely identify the dot node
			nodeId = utils.SSABlockString(block) + fmt.Sprintf("%p", n.insn)
		case AnySynthetic:
			nodeId = n.Id()
			if n.Block() != nil {
				block = n.Block()
			}
		default:
			// TODO
			continue
		}

		dnode := &dot.DotNode{
			// Make node IDs unique across functions (multiple functions share e.g. the "[ return ]" string).
			ID: fmt.Sprintf("%s-%s", funId, nodeId),
			Attrs: dot.DotAttrs{
				"label": node.String(),
			},
		}
		deferred := node.IsDeferred()
		if deferred {
			dnode.Attrs["fillcolor"] = "#a0ecfa"
		}

		if block != nil {
			blockId := utils.SSABlockString(block)
			if cluster.Clusters[blockId] == nil {
				cluster.Clusters[blockId] = dot.NewDotCluster(blockId)
				cluster.Clusters[blockId].Attrs["bgcolor"] = "#cce6ff"
				cluster.Clusters[blockId].Attrs["label"] = "Block " + strconv.Itoa(block.Index)
			}
			cluster.Clusters[blockId].Nodes = append(cluster.Clusters[blockId].Nodes, dnode)
		} else {
			cluster.Nodes = append(cluster.Nodes, dnode)
		}

		nodeToDotNode[node] = dnode
	}

	// Add intraprocedural edges
	for node := range nodesInFunc {
		// Edges for successors in the same function
		for next := range node.Successors() {
			if _, ok := nodesInFunc[next]; ok {
				addEdge(node, next, nil)
			}
		}
		for next := range node.Spawns() {
			if _, ok := nodesInFunc[next]; ok {
				addEdge(node, next, dot.DotAttrs{
					"style": "bold",
					"color": "sienna",
				})
			}
		}

		// Edge for post-call node
		if cr := node.CallRelation(); cr != nil {
			switch pcr := cr.(type) {
			case *CallNodeRelation:
				addEdge(node, pcr.post, dot.DotAttrs{
					"style": "dashed",
				})
			}
		}

		// Edge for defer node
		if def := node.DeferLink(); def != nil && !node.IsDeferred() {
			addEdge(node, def, dot.DotAttrs{
				"style": "dashed",
				"color": "blue",
			})
		}

		// Edges for panic continuation
		if pnc := node.PanicCont(); pnc != nil {
			addEdge(node, pnc, dot.DotAttrs{
				"style": "dashed",
				"color": "red",
			})
		}
	}

	return true
}
