package graph

import (
	"fmt"
	"strings"
	"testing"
)

func TestSCCToGraph(t *testing.T) {
	scc := _sampleGraph.SCC([]int{0})

	for _, comp := range scc.Components {
		fmt.Println("Component:", comp)
		strs := []string{}
		for _, e := range scc.Convolution().Edges(comp[0]) {
			strs = append(strs, fmt.Sprintf("%v", scc.Components[e]))
		}
		if len(strs) > 0 {
			fmt.Println("Edges to: ", strings.Join(strs, ", "))
		} else {
			fmt.Println("No edges")
		}
	}
}
