package graph

var edges = map[int][]int{
	0:  {1, 8},
	1:  {4, 5, 2},
	2:  {6, 3, 9},
	3:  {2, 7},
	4:  {0, 5},
	5:  {6},
	6:  {5},
	7:  {3, 6},
	8:  {},
	9:  {10, 11},
	10: {12, 13},
	11: {12, 13},
	12: {},
	13: {},
}
var _sampleGraph = OfHashable(func(i int) []int {
	return edges[i]
})
