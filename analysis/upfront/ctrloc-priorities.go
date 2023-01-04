package upfront

import (
	"github.com/cs-au-dk/goat/utils/graph"

	"golang.org/x/tools/go/ssa"
)

// CtrLocPriorities assigns functions to priorities and blocks within functions to priorities.
type CtrLocPriorities struct {
	FunPriorities   map[*ssa.Function]int
	BlockPriorities map[*ssa.Function][]int
}

// GetCtrLocPriorities uses the SCC decomposition of the callgraph to assign priorities to ssa.Functions.
// Functions are assigned priorities in depth-first order. We prioritize "deeper" functions before others.
// We cannot retrieve allFuns ourselves because importing cfg causes an import cycle.
func GetCtrLocPriorities(allFuns map[*ssa.Function]struct{}, scc graph.SCCDecomposition[*ssa.Function]) CtrLocPriorities {
	funPriorities := make(map[*ssa.Function]int, len(allFuns))
	blockPriorities := make(map[*ssa.Function][]int, len(allFuns))

	time := 0
	// Components are ordered in reverse topological order, so we reuse that ordering
	for _, component := range scc.Components {
		for _, node := range component {
			funPriorities[node] = time
			time++
		}
	}

	// NOTE: The pointer analysis and resulting callgraph is not sound, so some
	// functions may be missing a priority. Currently we accept that these
	// functions get negative priority (process first).
	time = 0
	for fun := range allFuns {
		if _, found := funPriorities[fun]; !found {
			time--
			funPriorities[fun] = time
		}

		if len(fun.Blocks) != 0 {
			domPreorder := fun.DomPreorder()
			bbGraph := graph.FromBasicBlocks(fun)
			bbSCC := bbGraph.SCC([]int{0})
			bprios := make([]int, len(fun.Blocks))
			bTime := 0

			// Assign block priorities in topological component order
			for compIdx := len(bbSCC.Components) - 1; compIdx >= 0; compIdx-- {
				component := bbSCC.Components[compIdx]
				if len(component) == 1 {
					bprios[component[0]] = bTime
					bTime++
				} else {
					// Sort basic blocks in a loop by dom-preorder
					inComponent := map[int]bool{}
					for _, bIdx := range component {
						inComponent[bIdx] = true
					}

					for _, b := range domPreorder {
						if inComponent[b.Index] {
							bprios[b.Index] = bTime
							bTime++
						}
					}
				}
			}

			/*
				nodes := []interface{}{}
				for i := range domPreorder {
					nodes = append(nodes, i)
				}
				bbGraph.ToDotGraph(nodes, &graph.VisualizationConfig{
					NodeAttrs: func(node interface{}) (string, dot.DotAttrs) {
						return fmt.Sprint(node), dot.DotAttrs{
							"label": fmt.Sprintf("%s\n%v - %d", fun.String(), node, bprios[node.(int)]),
						}
					},
				})
			*/

			blockPriorities[fun] = bprios
		}
	}

	return CtrLocPriorities{
		funPriorities, blockPriorities,
	}
}
