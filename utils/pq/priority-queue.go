package pq

import "container/heap"

type lessFunc[T any] func(T, T) bool

// A type satisfying the heap.Interface
type _heap[T any] struct {
	list []T
	less lessFunc[T]
}

func (h _heap[T]) Len() int {
	return len(h.list)
}

func (h _heap[T]) Swap(i, j int) {
	l := h.list
	l[i], l[j] = l[j], l[i]
}

func (h *_heap[T]) Push(x interface{}) {
	h.list = append(h.list, x.(T))
}

func (h *_heap[T]) Pop() interface{} {
	old := h.list
	n := len(old)
	x := old[n-1]
	h.list = old[0 : n-1]
	return x
}

func (h _heap[T]) Less(i, j int) bool {
	return h.less(h.list[i], h.list[j])
}

var _ heap.Interface = (*_heap[int])(nil)

type PriorityQueue[T any] struct {
	heap     _heap[T]
	// TODO: CtrLoc is not comparable (because cfg.Node is an interface?), so
	// we cannot use T as a key for the elements map.
	elements map[interface{}]struct{}
}

func Empty[T any](less lessFunc[T]) PriorityQueue[T] {
	return PriorityQueue[T]{
		heap:     _heap[T]{nil, less},
		elements: make(map[interface{}]struct{}),
	}
}

func (p *PriorityQueue[T]) IsEmpty() bool {
	return len(p.heap.list) == 0
}

func (p *PriorityQueue[T]) GetNext() T {
	el := heap.Pop(&p.heap).(T)
	delete(p.elements, el)
	return el
}

func (p *PriorityQueue[T]) Add(x T) {
	if _, found := p.elements[x]; found {
		return
	}

	p.elements[x] = struct{}{}
	heap.Push(&p.heap, x)
}

func (p *PriorityQueue[T]) Rebuild() {
	heap.Init(&p.heap)
}
