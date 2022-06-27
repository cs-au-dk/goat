package worklist

import "sync"

type Worklist[T any] struct {
	list []T
	mu   sync.Mutex
}

// Start worklist execution with provided `starting` element and an iteration
// function. The iteration function exposes the next element and a function with
// which to add more elements to the worklist.
func Start[T any](start T, do func(next T, add func(el T))) {
	StartV([]T{start}, do)
}

// Start worklist execution with a preloaded queue and an iteration
// function. The iteration function exposes the next element and a function with
// which to add more elements to the worklist.
func StartV[T any](start []T, do func(next T, add func(el T))) {
	W := Empty[T]()
	for _, e := range start {
		W.Add(e)
	}

	W.Process(do)
}

func Empty[T any]() Worklist[T] {
	return Worklist[T]{}
}

func (w *Worklist[T]) GetNext() (ret T) {
	if len(w.list) == 0 {
		return
	}
	next := w.list[0]
	w.list = w.list[1:]
	return next
}

func (w *Worklist[T]) IsEmpty() bool {
	return len(w.list) == 0
}

func (w *Worklist[T]) Process(
	do func(
		next T,
		add func(element T))) {
	for !w.IsEmpty() {
		do(w.GetNext(), w.Add)
	}
}

func (w *Worklist[T]) ProcessConc(
	start T,
	do func(
		next T,
		add func(el T))) {
	w.AddConc(start)
	for !w.IsEmptyConc() {
		do(w.GetNextConc(), w.Add)
	}
}

func (w *Worklist[T]) Add(el T) {
	w.list = append(w.list, el)
}

func (w *Worklist[T]) AddConc(el T) {
	w.mu.Lock()
	w.Add(el)
	w.mu.Unlock()
}

func (w *Worklist[T]) GetNextConc() T {
	w.mu.Lock()
	next := w.GetNext()
	w.mu.Unlock()
	return next
}

func (w *Worklist[T]) IsEmptyConc() bool {
	w.mu.Lock()
	empty := w.IsEmpty()
	w.mu.Unlock()
	return empty
}
