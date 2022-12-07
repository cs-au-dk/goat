package graph

// A DAG decomposition of a graph based on strongly connected components.
// The nodes in component i are guaranteed to only have edges to nodes in
// components with index j <= i.
type SCCDecomposition[T any] struct {
	Components  [][]T
	comp        Mapper[T]
	convolution *WrappedGraph[SCC, T]
	Original    Graph[T]
}

// An alias for component type (in case representation changes)
type SCC = int

// Returns the index of the component the node is a part of.
func (scc SCCDecomposition[T]) ComponentOf(node T) SCC {
	if comp, hasComp := scc.comp.Get(node); hasComp {
		return comp.(int)
	}

	// TODO: Change return type to (int, err) or (int, bool) instead and introduce
	// UnsafeComponentOf?
	//panic(fmt.Errorf("%v does not belong to a component", node))
	return -1
}

// Compute the strongly connected components of the subgraph reachable from the
// provided start nodes.
func (G Graph[T]) SCC(startNodes []T) SCCDecomposition[T] {
	// Source:
	// https://github.com/kth-competitive-programming/kactl/blob/main/content/graph/SCC.h

	val, comp := G.mapFactory(), G.mapFactory()
	time := 0
	var z, cont []T
	var components [][]T

	var rec func(T)
	rec = func(node T) {
		time++
		low := time
		val.Set(node, low)
		stackH := len(z)
		z = append(z, node)

		for _, e := range G.Edges(node) {
			if _, hasComp := comp.Get(e); !hasComp {
				if _, visited := val.Get(e); !visited {
					rec(e)
				}

				eLow, _ := val.Get(e)
				if eLow.(int) < low {
					low = eLow.(int)
				}
			}
		}

		if oldLow, _ := val.Get(node); low == oldLow.(int) {
			for len(z) > stackH {
				x := z[len(z)-1]
				z = z[:len(z)-1]
				comp.Set(x, len(components))
				cont = append(cont, x)
			}

			components = append(components, cont)
			cont = nil
		}

		val.Set(node, low)
	}

	for _, node := range startNodes {
		if _, hasComp := comp.Get(node); !hasComp {
			rec(node)
		}
	}

	return SCCDecomposition[T]{
		Components:  components,
		comp:        comp,
		convolution: nil,
		Original:    G,
	}
}

// Construct SCC convolution as a wrapped Graph.
func (scc SCCDecomposition[T]) Convolution() WrappedGraph[SCC, T] {
	if scc.convolution == nil {
		scc.convolution = new(WrappedGraph[SCC, T])
		*scc.convolution = WrappedGraph[SCC, T]{
			OfHashable(
				// Get SCC edges based on component index
				func(compid SCC) (ret []SCC) {
					comp := scc.Components[compid]
					found := make(map[SCC]struct{})
					for _, n := range comp {
						for _, e := range scc.Original.Edges(n) {
							if compid2 := scc.ComponentOf(e); compid != compid2 {
								found[compid2] = struct{}{}
							}
						}
					}

					for comp := range found {
						ret = append(ret, comp)
					}
					return
				},
			),
			// Get SCC edges based on original graph nodes
			func(node T) (ret []SCC) {
				// The children of a SCC are all the other SCCs of nodes to
				// which nodes in the component might have an edge to.
				compid := scc.ComponentOf(node)
				comp := scc.Components[compid]
				found := make(map[SCC]struct{})
				for _, n := range comp {
					for _, e := range scc.Original.Edges(n) {
						if compid2 := scc.ComponentOf(e); compid != compid2 {
							found[compid2] = struct{}{}
						}
					}
				}

				for comp := range found {
					ret = append(ret, comp)
				}
				return
			},
			scc.Original.mapFactory(),
		}
	}

	return *scc.convolution
}

// A wrapped graph uses a second strategy to access nodes. The overriding edgesOf
// reflects this strategy, with the overriding cachedEdges caching these results,
// while the underlying graph still provides access to regular edgesOf and
// cachedEdges. The embedded graph can be reached via .Underlying()
//
// Example: SCC convolutions can collect and cache edges using nodes in the
// original graph, by using the edgesOf created during SCC decomposition
// in the wrapped graph. The underlying graph access and caches edges based on SCC indexes.
type WrappedGraph[W, T any] struct {
	Graph[W]
	edgesOf     func(T) []W
	cachedEdges Mapper[T]
}


func (G WrappedGraph[W, T]) Edges(node T) []W {
	if cached, found := G.cachedEdges.Get(node); found {
		return cached.([]W)
	}

	es := G.edgesOf(node)
	G.cachedEdges.Set(node, es)
	return es
}

func (G WrappedGraph[W, T]) Underlying() Graph[W] {
	return G.Graph
}

// Returns a graph based on the SCC decomposition.
// Nodes are component indices (int).
func (scc SCCDecomposition[T]) ToGraph() Graph[SCC] {
	return OfHashable(func(compIdx SCC) (ret []SCC) {
		seen := map[int]bool{}
		for _, node := range scc.Components[compIdx] {
			for _, edge := range scc.Original.Edges(node) {
				ncomp := scc.ComponentOf(edge)
				if compIdx != ncomp && !seen[ncomp] {
					seen[ncomp] = true
					ret = append(ret, ncomp)
				}
			}
		}
		return
	})
}
