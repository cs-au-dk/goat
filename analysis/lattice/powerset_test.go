package lattice

import "testing"

func TestPowersetComparison(t *testing.T) {
	powLat := Create().Lattice().PowersetVariadic("a", "b", "c")
	elFactory := Create().Element().Powerset(powLat)

	a := elFactory(set{"a": true})
	b := elFactory(set{"b": true})
	c := elFactory(set{"c": true})
	ab := elFactory(set{"a": true, "b": true})

	tests := []struct {
		a, b      Element
		predicate func(Element) bool
		symbol    string
		expected  bool
	}{
		{powLat.Bot(), powLat.Bot(), powLat.Bot().Eq, "=", true},
		{powLat.Top(), powLat.Top(), powLat.Top().Eq, "=", true},
		{a, a, a.Leq, "⊑", true},
		{a, a, a.Geq, "⊒", true},
		{a, b, a.Geq, "⊒", false},
		{a, b, a.Leq, "⊑", false},
		{a, ab, a.Leq, "⊑", true},
		{b, ab, b.Leq, "⊑", true},
		{c, ab, c.Leq, "⊑", false},
		{ab, powLat.Top(), ab.Leq, "⊑", true},
		{ab, powLat.Top(), ab.Geq, "⊒", false},
		{ab, powLat.Bot(), ab.Leq, "⊑", false},
		{ab, powLat.Bot(), ab.Geq, "⊒", true},
	}

	for _, test := range tests {
		res := test.predicate(test.b)
		if res != test.expected {
			t.Errorf("%s %s %s = %v, expected %v\n", test.a, test.symbol, test.b, res, test.expected)
		} else {
			t.Logf("%s %s %s = %v\n", test.a, test.symbol, test.b, res)
		}
	}
}

func TestPowersetJoin(t *testing.T) {
	powLat := Create().Lattice().PowersetVariadic("a", "b", "c")
	elFactory := Create().Element().Powerset(powLat)
	a := elFactory(set{"a": true})
	b := elFactory(set{"b": true})
	c := elFactory(set{"c": true})
	ab := elFactory(set{"a": true, "b": true})

	tests := []struct {
		a, b, expected Element
	}{
		{powLat.Bot(), powLat.Bot(), powLat.Bot()},
		{powLat.Top(), powLat.Top(), powLat.Top()},
		{powLat.Bot(), powLat.Top(), powLat.Top()},
		{powLat.Top(), powLat.Bot(), powLat.Top()},
		{powLat.Bot(), a, a},
		{a, powLat.Bot(), a},
		{b, powLat.Bot(), b},
		{c, powLat.Bot(), c},
		{a, a, a},
		{a, b, ab},
		{b, a, ab},
		{b, ab, ab},
		{c, ab, powLat.Top()},
		{ab, powLat.Top(), powLat.Top()},
	}

	for _, test := range tests {
		res := test.a.Join(test.b)
		if !res.Eq(test.expected) {
			t.Errorf("%s ⊔ %s = %s, expected %s\n", test.a, test.b, res, test.expected)
		} else {
			t.Logf("%s ⊔ %s = %s\n", test.a, test.b, res)
		}
	}
}
