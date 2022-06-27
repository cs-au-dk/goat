package graph

import (
	"fmt"
	"testing"
)

func TestOLCA(t *testing.T) {
	scc := _sampleGraph.SCC([]int{0})

	G := scc.ToGraph()

	lca := G.FullTarjanOLCA(len(scc.Components) - 1)
	for n, rep := range lca.reps {
		fmt.Println(n, scc.Components[n.(int)], rep.ancestor.node, scc.Components[rep.ancestor.node.(int)])
	}
}
