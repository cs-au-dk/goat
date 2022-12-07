package graph

import (
	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/ssa"
)

const CGPruneLimit = 10

// Creates a Graph from a callgraph with *ssa.Functions as nodes.
// Duplicate edges in the callgraph are pruned.
// If prune is true edges from call sites with at least 10 targets
// will not be included in the resulting graph.
func FromCallGraph(cg *callgraph.Graph, prune bool) Graph[*ssa.Function] {
	return OfHashable(func(fun *ssa.Function) (ret []*ssa.Function) {
		// NOTE: We unsoundly prune call edges from the graph that originate
		// from sites with >= 10 call targets. This makes our heuristics for
		// computing fragments based on an SCC decomposition of the call graph
		// more precise.
		// TODO (O): This is necesary for the Kubernetes or gRPC benchmark.
		// Investigate and document which one it is.
		siteCnt := map[ssa.CallInstruction]int{}
		if _, found := cg.Nodes[fun]; !found {
			return
		}

		for _, edge := range cg.Nodes[fun].Out {
			siteCnt[edge.Site]++
		}

		dedup := map[*ssa.Function]bool{}
		for _, edge := range cg.Nodes[fun].Out {
			if seen := dedup[edge.Callee.Func]; !seen &&
				(!prune || siteCnt[edge.Site] < CGPruneLimit) {
				dedup[edge.Callee.Func] = true
				ret = append(ret, edge.Callee.Func)
			}
		}
		return
	})
}

// Nodes are BB indices.
func FromBasicBlocks(fun *ssa.Function) Graph[int] {
	return OfHashable(func(node int) (ret []int) {
		bb := fun.Blocks[node]
		for _, succ := range bb.Succs {
			ret = append(ret, succ.Index)
		}
		return
	})
}
