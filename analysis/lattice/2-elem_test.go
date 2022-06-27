package lattice

import "testing"

var bot Element = Create().Lattice().TwoElement().Bot()
var top Element = Create().Lattice().TwoElement().Top()

func TestTwoElementJoin(t *testing.T) {
	tests := []struct{ a, b, expected Element }{
		{bot, bot, bot},
		{bot, top, top},
		{top, bot, top},
		{top, top, top},
	}

	for _, test := range tests {
		res := test.a.Join(test.b)
		if res != test.expected {
			t.Errorf("%s ⊔ %s = %s, expected %s\n", test.a, test.b, res, test.expected)
		} else {
			t.Logf("%s ⊔ %s = %s\n", test.a, test.b, res)
		}
	}
}

func TestTwoElementLeq(t *testing.T) {
	tests := []struct {
		a, b     Element
		expected bool
	}{
		{bot, bot, true},
		{bot, top, true},
		{top, bot, false},
		{top, top, true},
	}

	for _, test := range tests {
		res := test.a.Leq(test.b)
		if res != test.expected {
			t.Errorf("%s ⊑ %s = %v, expected %v\n", test.a, test.b, res, test.expected)
		} else {
			t.Logf("%s ⊑ %s = %v\n", test.a, test.b, res)
		}
	}
}
