package tree

// zeroBit checks whether a key is the 0 bit at a given branching point.
func zeroBit(key, bit keyt) bool {
	return key&bit == 0
}

// branchingBit is the lowest prefix where a branching occurs.
func branchingBit(p0, p1 keyt) keyt {
	diff := p0 ^ p1
	return diff & -diff
}
