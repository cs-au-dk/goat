package lattice

import "testing"

func TestIntervalJoin(t *testing.T) {
	lat := Create().Lattice().Interval()
	int := Create().Element().Interval

	type b = FiniteBound
	type P = PlusInfinity
	type M = MinusInfinity

	tests := []struct {
		a, b, expected Element
	}{
		{lat.Bot(), lat.Bot(), lat.Bot()},
		{lat.Bot(), lat.Top(), lat.Top()},
		{lat.Top(), lat.Bot(), lat.Top()},
		{lat.Top(), lat.Top(), lat.Top()},
		{lat.Bot(), int(b(0), b(0)), int(b(0), b(0))},
		{int(b(0), b(0)), lat.Bot(), int(b(0), b(0))},
		{int(b(0), b(0)), int(b(1), b(1)), int(b(0), b(1))},
		{int(b(1), b(1)), int(b(0), b(0)), int(b(0), b(1))},
		{int(b(1), b(2)), int(b(3), b(4)), int(b(1), b(4))},
		{int(b(-1), b(0)), int(b(0), b(1)), int(b(-1), b(1))},
		{int(b(0), b(1)), int(b(-1), b(0)), int(b(-1), b(1))},
		{int(b(0), b(1024)), int(b(0), P{}), int(b(0), P{})},
		{int(b(0), P{}), int(b(0), b(1024)), int(b(0), P{})},
		{int(b(-1024), b(0)), int(b(0), P{}), int(b(-1024), P{})},
		{int(M{}, b(0)), int(b(-1024), b(0)), int(M{}, b(0))},
		{int(b(-1024), b(0)), int(M{}, b(0)), int(M{}, b(0))},
		{int(M{}, b(-1024)), int(b(1024), P{}), lat.Top()},
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
