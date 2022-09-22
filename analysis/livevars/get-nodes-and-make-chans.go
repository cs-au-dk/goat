package livevars

import (
	"github.com/cs-au-dk/goat/analysis/cfg"

	"golang.org/x/tools/go/ssa"
)

var chans = make(map[interface{}]bool)
var nodes = make(map[interface{}]bool)

func getAllMakeChans(G cfg.Cfg) {
	var visit func(cfg.Node)
	visit = func(n cfg.Node) {
		if _, ok := nodes[n]; !ok {
			nodes[n] = true
			switch n := n.(type) {
			case *cfg.SSANode:
				switch i := n.Instruction().(type) {
				case *ssa.MakeChan:
					chans[i] = true
				}
			}

			for succ := range n.Successors() {
				visit(succ)
			}
			for pred := range n.Predecessors() {
				visit(pred)
			}
			for spawn := range n.Spawns() {
				visit(spawn)
			}
		}
	}

	for _, entry := range G.GetEntries() {
		visit(entry)
	}
}
