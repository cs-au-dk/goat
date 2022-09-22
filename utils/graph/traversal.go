package graph

import W "github.com/cs-au-dk/goat/utils/worklist"

type traversalFunc[T any] func(node T) (stop bool)

// Performs a breadth-first search from the provided start nodes, calling the
// provided function (f) for every reachable node, stopping early if f returns
// true.
// Returns whether the search stopped early (as a result of f returning true).
func (G Graph[T]) BFSV(f traversalFunc[T], starts ...T) bool {
	visited := G.mapFactory()
	for _, start := range starts {
		visited.Set(start, true)
	}

	done := false
	W.StartV(starts, func(node T, add func(T)) {
		if done || f(node) {
			done = true
			return
		}

		for _, next := range G.Edges(node) {
			if _, found := visited.Get(next); !found {
				visited.Set(next, true)
				add(next)
			}
		}
	})

	return done
}

// Performs a breadth-first search from the provided start node, calling the
// provided function (f) for every reachable node, stopping early if f returns
// true.
// Returns whether the search stopped early (as a result of f returning true).
func (G Graph[T]) BFS(start T, f traversalFunc[T]) bool {
	return G.BFSV(f, start)
}
