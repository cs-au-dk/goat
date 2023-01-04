package pq

import "container/heap"

// lessFunc is a comparison function between two elements of type T.
type lessFunc[T any] func(T, T) bool

// _heap satisfies the heap.Interface. It includes a list of elements,
// and a comparison function.
type _heap[T any] struct {
	list []T
	less lessFunc[T]
}

// Len returns the size of the heap.
func (h _heap[T]) Len() int {
	return len(h.list)
}

// Swap interchanges the values of the elements at the given indices.
func (h _heap[T]) Swap(i, j int) {
	l := h.list
	l[i], l[j] = l[j], l[i]
}

// Push appends a given element to the heap.
func (h *_heap[T]) Push(x any) {
	h.list = append(h.list, x.(T))
}

// Pop retrieves the last element in the heap.
func (h *_heap[T]) Pop() any {
	old := h.list
	n := len(old)
	x := old[n-1]
	h.list = old[0 : n-1]
	return x
}

// Less compares two elements in the heap at the given indices.
func (h _heap[T]) Less(i, j int) bool {
	return h.less(h.list[i], h.list[j])
}

var _ heap.Interface = (*_heap[int])(nil)

// PriorityQueue implements a priority queue.
type PriorityQueue[T any] struct {
	heap _heap[T]
	// TODO: CtrLoc is not comparable (because cfg.Node is an interface?), so
	// we cannot use T as a key for the elements map.
	elements map[any]struct{}
}

// Empty creates an empty priority queue for elements of a given type,
// with the given comparison function.
func Empty[T any](less lessFunc[T]) PriorityQueue[T] {
	return PriorityQueue[T]{
		heap:     _heap[T]{nil, less},
		elements: make(map[interface{}]struct{}),
	}
}

// IsEmpty checks whether the priority queue is empty.
func (p *PriorityQueue[T]) IsEmpty() bool {
	return len(p.heap.list) == 0
}

// GetNext pops the top element from the heap.
func (p *PriorityQueue[T]) GetNext() T {
	el := heap.Pop(&p.heap).(T)
	delete(p.elements, el)
	return el
}

// Add inserts the given element in the heap, if not already present.
func (p *PriorityQueue[T]) Add(x T) {
	if _, found := p.elements[x]; found {
		return
	}

	p.elements[x] = struct{}{}
	heap.Push(&p.heap, x)
}

// Rebuild re-establishes all the invariants of the heap.
func (p *PriorityQueue[T]) Rebuild() {
	heap.Init(&p.heap)
}
