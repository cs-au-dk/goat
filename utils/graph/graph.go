package graph

/*
	This package exposes utilities for working with graph structures.

	Graph structures appear in various places in this project and has prompted many
	ad-hoc implementations of standard graph algorithms.

	The goal of this package is to provide easy access to graph algorithms on
	data that has a graph representation.
	Currently this is done by only requiring the caller to provide a function
	describing the edge relation (and a key-value map factory for the node type).
*/

type Mapper[K any] interface {
	Get(key K) (any, bool)
	Set(key K, value any)
}

// TODO: There's currently no way to specify an additional type parameter on
// the function type for the types of values in the map.
type mapFactory[K any] func() Mapper[K]
type edgesOf[T any] func(node T) []T

type Graph[T any] struct {
	mapFactory  mapFactory[T]
	edgesOf     edgesOf[T]
	cachedEdges Mapper[T]
}

func (G Graph[T]) Edges(node T) []T {
	if cached, found := G.cachedEdges.Get(node); found {
		return cached.([]T)
	}

	es := G.edgesOf(node)
	G.cachedEdges.Set(node, es)
	return es
}

func Of[T any](mapFactory mapFactory[T], edgesOf edgesOf[T]) Graph[T] {
	return Graph[T]{
		mapFactory,
		edgesOf,
		mapFactory(),
	}
}

// Mapper implementation using Go's builtin maps
type mapMapper[K comparable] map[K]any

func (m mapMapper[K]) Get(key K) (any, bool) {
	value, ok := m[key]
	return value, ok
}

func (m mapMapper[K]) Set(key K, value any) {
	m[key] = value
}

func OfHashable[K comparable](edgesOf edgesOf[K]) Graph[K] {
	return Of(func() Mapper[K] { return mapMapper[K]{} }, edgesOf)
}
