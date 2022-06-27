package tree

func zeroBit(key, bit keyt) bool {
	return key & bit == 0
}

func branchingBit(p0, p1 keyt) keyt {
	diff := p0 ^ p1
	return diff & -diff
}
