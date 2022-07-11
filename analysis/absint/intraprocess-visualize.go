package absint

import (
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	"Goat/utils"
	"Goat/utils/dot"
	"fmt"
	"strconv"

	"golang.org/x/tools/go/ssa"
)

// Debugging utility I made to visualize results of intraprocedural abstract interpretation
// If -verbose is specified the SSA-nodes will be annotated with abstract values for operands.
func VisualizeIntraprocess(
	g defs.Goro,
	edges map[defs.CtrLoc][]defs.CtrLoc,
	analysis map[defs.CtrLoc]L.AnalysisState,
) {
	G := &dot.DotGraph{
		Options: map[string]string{
			"minlen":  fmt.Sprint(opts.Minlen()),
			"nodesep": fmt.Sprint(opts.Nodesep()),
			"rankdir": "TB",
			"label":   "Debug",
		},
	}

	funToDotCluster := map[*ssa.Function]*dot.DotCluster{}
	getCluster := func(fun *ssa.Function) *dot.DotCluster {
		cluster, found := funToDotCluster[fun]
		if !found {
			funId := utils.SSAFunString(fun)
			cluster = dot.NewDotCluster(funId)
			cluster.Attrs["label"] = funId
			cluster.Attrs["bgcolor"] = "#e6ffff"

			funToDotCluster[fun] = cluster
			G.Clusters = append(G.Clusters, cluster)
		}

		return cluster
	}

	clToDotNode := map[defs.CtrLoc]*dot.DotNode{}
	getNode := func(cl defs.CtrLoc) *dot.DotNode {
		node, found := clToDotNode[cl]
		if !found {
			label := cl.String()
			if !cl.Panicked() {
				label += "\n" + StringifyNodeArguments(g, analysis[cl].Stack(), cl.Node())
			}

			node = &dot.DotNode{
				ID: strconv.Itoa(int(cl.Hash())),
				Attrs: dot.DotAttrs{
					"label": label,
				},
			}

			if cl.Node().IsDeferred() {
				node.Attrs["fillcolor"] = "#a0ecfa"
			}

			clToDotNode[cl] = node
			C := getCluster(cl.Node().Function())
			C.Nodes = append(C.Nodes, node)
		}

		return node
	}

	for cl, eds := range edges {
		for _, ncl := range eds {
			edge := &dot.DotEdge{
				From:  getNode(cl),
				To:    getNode(ncl),
				Attrs: dot.DotAttrs{},
			}

			if !cl.Panicked() && ncl.Panicked() {
				// Drop panic-edges for moderately sized graphs.
				if len(edges) > 100 {
					continue
				}
				edge.Attrs["color"] = "red"
			}

			G.Edges = append(G.Edges, edge)
		}

		// Edge for post-call node
		if cr := cl.Node().CallRelation(); cr != nil {
			switch cr.(type) {
			case *cfg.CallNodeRelation:
				pcl := cl.Derive(cl.Node().CallRelationNode())
				if _, found := edges[pcl]; found {
					G.Edges = append(G.Edges, &dot.DotEdge{
						From: getNode(cl),
						To:   getNode(pcl),
						Attrs: dot.DotAttrs{
							"style":      "dashed",
							"constraint": "false",
						},
					})
				}
			}
		}
	}

	G.ShowDot()
}

func StringifyNodeArguments(g defs.Goro, stack L.AnalysisStateStack, node cfg.Node) (label string) {
	var ops []ssa.Value
	if n, ok := node.(*cfg.SSANode); ok {
		for _, pval := range n.Instruction().Operands(nil) {
			ops = append(ops, *pval)
		}
	} else if n, ok := node.(*cfg.BuiltinCall); ok {
		ops = n.Args()
	}

	for _, op := range ops {
		if op != nil {
			label += fmt.Sprintf("%s = ", op.Name())
			func() {
				defer func() {
					if err := recover(); err != nil {
						label += "Not found"
					}
				}()

				label += evaluateSSA(g, stack, op).String()
			}()
			label += "\n"
		}
	}

	return label
}
