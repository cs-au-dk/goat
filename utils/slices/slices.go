package slices

// Find searches for a given element in a slice of elements of the same type.
// It relaxes comparison between primitives with underlying types.
func Find[E ~[]T, T any](l E, pred func(T) bool) (T, bool) {
	for _, x := range l {
		if pred(x) {
			return x, true
		}
	}
	var x T
	return x, false
}

func OneOf[T comparable](x T, xs ...T) bool {
	for _, x2 := range xs {
		if x == x2 {
			return true
		}
	}

	return false
}
