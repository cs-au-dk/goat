package set

type subsets []interface{}

func Subsets(entries []interface{}) subsets {
	return entries
}

func SubsetsV(entries ...interface{}) subsets {
	return entries
}

func (S subsets) ForEach(do func([]interface{})) {
	last := len(S) - 1

	ss := []int{}

	for ss != nil {
		subset := make([]interface{}, 0, len(S))

		for _, i := range ss {
			subset = append(subset, S[i])
		}

		do(subset)

		switch {
		// Initial set is empty
		case len(S) == 0:
			ss = nil
		// Process the empty subset
		case len(ss) == 0:
			new := append(ss, 0)
			ss = new
		// If the subset is a singleton and the processed element
		// was the last in the original set, we are done.
		case len(ss) == 1 && ss[0] == last:
			ss = nil
		// If the last element in the subset is the last element in
		// the original set, then discard it, and increment the index on the second
		// to last element.
		case ss[len(ss)-1] == last:
			ss = append(ss[:len(ss)-2], ss[len(ss)-2]+1)
		// Otherwise, add the next element to the list so far.
		default:
			ss = append(ss, ss[len(ss)-1]+1)
		}
	}
}
