package graph

import "fmt"

type lcaRep struct {
	node            interface{}
	parent          *lcaRep
	ancestor        *lcaRep
	rank            int
	childrenVisited bool
}

type LCA[T any] struct {
	Pairs map[interface{}]set
	reps  map[interface{}]*lcaRep
	G     Graph[T]
}

// Instantiate node in representative map of Union-Find
// data structure
func (lca LCA[T]) MakeSet(node interface{}) {
	var rep *lcaRep
	if r, found := lca.reps[node]; !found {
		rep = new(lcaRep)
		rep.node = node
		lca.reps[node] = rep
	} else {
		rep = r
	}

	rep.parent = rep
	rep.rank = 1
}

func (lca LCA[T]) Find(node interface{}) *lcaRep {
	// Optimistically assume all Finds precede all MakeSets
	rep, ok := lca.reps[node]
	if !ok {
		lca.MakeSet(node)
		rep = lca.reps[node]
	}

	if rep.parent != rep {
		rep.parent = lca.Find(rep.parent.node)
	}

	return rep.parent
}

func (lca LCA[T]) Union(x, y interface{}) {
	xRep := lca.Find(x)
	yRep := lca.Find(y)

	switch {
	case xRep.rank > yRep.rank:
		yRep.parent = xRep
	case xRep.rank < yRep.rank:
		xRep.parent = yRep
	default:
		yRep.parent = xRep
		xRep.rank = xRep.rank + 1
	}
}

type set = map[interface{}]struct{}

func (G Graph[T]) FullTarjanOLCA(root T) LCA[T] {
	visited := make(map[interface{}]struct{})
	var rec func(T)
	rec = func(node T) {
		if _, ok := visited[node]; ok {
			return
		}

		visited[node] = struct{}{}
		for _, n := range G.edgesOf(node) {
			rec(n)
		}
	}
	rec(root)

	P := make(map[interface{}]set)
	for n1 := range visited {
		if _, ok := P[n1]; !ok {
			P[n1] = make(set)
		}

		for n2 := range visited {
			if n1 == n2 {
				continue
			}

			P[n1][n2] = struct{}{}
		}
	}

	return G.TarjanOLCA(root, P)
}

func (G Graph[T]) TarjanOLCA(root T, P map[interface{}]set) LCA[T] {
	lca := LCA[T]{
		Pairs: P,
		reps:  make(map[interface{}]*lcaRep),
		G:     G,
	}

	lca.TarjanOLCA(root)
	return lca
}

func (lca LCA[T]) TarjanOLCA(u T) {
	lca.MakeSet(u)
	rep := lca.reps[u]
	rep.ancestor = rep
	for _, v := range lca.G.edgesOf(u) {
		lca.TarjanOLCA(v)
		lca.Union(u, v)

		lca.Find(u).ancestor = rep
	}

	rep.childrenVisited = true
	for v := range lca.Pairs[u] {
		if r, ok := lca.reps[v]; ok && r.childrenVisited {
			fmt.Printf("Tarjan's LCA of %v and %v is %v\n", u, v, lca.Find(v).ancestor.node)
		}
	}
}
