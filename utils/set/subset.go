package set

// subsets computes all the given subsets for a given set of values,
// as a slice.
type subsets[T any] []T

// Subsets converts a slice to a set for which to compute subsets.
func Subsets[T any](entries []T) subsets[T] {
	return entries
}

// SubsetsV is a variadic conversion from a list of elements to a set
// for which to compute subsets.
func SubsetsV[T any](entries ...T) subsets[T] {
	return entries
}

// ForEach executes a procedure for each given subset of a given set.
func (S subsets[T]) ForEach(do func([]T)) {
	last := len(S) - 1
	ss := []int{}

	for ss != nil {
		subset := make([]T, 0, len(S))

		for _, i := range ss {
			subset = append(subset, S[i])
		}

		do(subset)

		switch {
		// Initial set is empty. Only the empty set is a viable subset,
		// and was already computed.
		case len(S) == 0:
			ss = nil
		// Initial set is not empty. The empty set was processed,
		// and now only non-empty sets need to be processed.
		case len(ss) == 0:
			new := append(ss, 0)
			ss = new
		// If the subset is a singleton and the processed element
		// was the last in the original set, we are done.
		case len(ss) == 1 && ss[0] == last:
			ss = nil
		// If the last element in the subset is the last element in the original set,
		// discard it, and increment the index on the second to last element.
		case ss[len(ss)-1] == last:
			ss = append(ss[:len(ss)-2], ss[len(ss)-2]+1)
		// Otherwise, add the next element to the list so far.
		default:
			ss = append(ss, ss[len(ss)-1]+1)
		}
	}
}
