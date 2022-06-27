package graph

import (
	"Goat/utils/dot"
	"Goat/utils"
	"fmt"
)

var opts = utils.Opts()

type VisualizationConfig[T any] struct {
	// Provides the ID and attributes for dot nodes.
	// If not provided, the ID is the stringified node.
	NodeAttrs func(node T) (string, dot.DotAttrs)
	// If provided, will create clusters for nodes with the same key.
	// The returned key must be safe to use in a Go map.
	ClusterKey func(node T) any
	// Provides the ID and attributes for dot clusters.
	ClusterAttrs func(key any) (string, dot.DotAttrs)
}

// TODO: Option to compute transitive closure from `nodes`
// TODO: Edge customization (requires modification of Graph type)
func (G Graph[T]) ToDotGraph(nodes []T, cfg *VisualizationConfig[T]) *dot.DotGraph {
	if cfg == nil {
		cfg = &VisualizationConfig[T]{}
	}

	graphOpts := map[string]string{
		"minlen":  fmt.Sprint(opts.Minlen()),
		"nodesep": fmt.Sprint(opts.Nodesep()),
		"rankdir": "TB",
	}

	if cfg.ClusterKey != nil {
		// Necessary to ensure that ordering within clusters is kept.
		// See https://stackoverflow.com/questions/33959969/order-cluster-nodes-in-graphviz
		//graphOpts["remincross"] = "false"
	}

	dg := &dot.DotGraph{
		Options: graphOpts,
	}

	keyToCluster := map[interface{}]*dot.DotCluster{}
	getCluster := func(key interface{}) *dot.DotCluster {
		if cluster, found := keyToCluster[key]; found {
			return cluster
		}

		var id string
		var attrs dot.DotAttrs
		if cfg.ClusterAttrs != nil {
			id, attrs = cfg.ClusterAttrs(key)
		} else {
			id = fmt.Sprint(key)
		}

		cluster := dot.NewDotCluster(id)
		cluster.Attrs = attrs
		dg.Clusters = append(dg.Clusters, cluster)

		keyToCluster[key] = cluster
		return cluster
	}

	// Add nodes to graph
	nodeToDotNode := G.mapFactory()
	for _, node := range nodes {
		dNode := &dot.DotNode{}

		if cfg.NodeAttrs != nil {
			dNode.ID, dNode.Attrs = cfg.NodeAttrs(node)
		} else {
			dNode.ID = fmt.Sprint(node)
		}

		nodeToDotNode.Set(node, dNode)

		if cfg.ClusterKey != nil {
			cl := getCluster(cfg.ClusterKey(node))
			cl.Nodes = append(cl.Nodes, dNode)
		} else {
			dg.Nodes = append(dg.Nodes, dNode)
		}
	}

	// Add edges to graph
	for _, node := range nodes {
		a, _ := nodeToDotNode.Get(node)

		for _, edge := range G.Edges(node) {
			if b, found := nodeToDotNode.Get(edge); found {
				dg.Edges = append(dg.Edges, &dot.DotEdge{
					From: a.(*dot.DotNode),
					To:   b.(*dot.DotNode),
				})
			}
		}
	}

	return dg
}
