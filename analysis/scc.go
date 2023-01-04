package analysis

import "github.com/cs-au-dk/goat/utils/graph"

// Bottom up analysis in SCCs.
func SCCAnalysis[Fact, T any](
	scc graph.SCCDecomposition[T],
	generateFact func(T) Fact,
	joinFacts func(Fact, Fact) Fact,
) []Fact {
	N := len(scc.Components)
	facts := make([]Fact, N)
	conv := scc.ToGraph()
	for ci, comp := range scc.Components {
		var fact Fact
		for i, node := range comp {
			genFact := generateFact(node)
			if i == 0 {
				fact = genFact
			} else {
				fact = joinFacts(fact, genFact)
			}

			for _, cj := range conv.Edges(ci) {
				if ci != cj {
					fact = joinFacts(fact, facts[cj])
				}
			}
		}

		facts[ci] = fact
	}
	return facts
}
