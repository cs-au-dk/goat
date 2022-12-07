package lattice

import "testing"

func TestLiftComparison(t *testing.T) {
	var T = Create().Lattice().TwoElement().Top()

	mapLat := MakeMapLatticeVariadic[string](Create().Lattice().TwoElement(), "a", "b", "c")
	liftedMapLat := Lift(mapLat)
	elFactory := MakeMap[string](mapLat)

	aT := elFactory(map[interface{}]Element{
		"a": T,
	})
	bT := elFactory(map[interface{}]Element{
		"b": T,
	})
	cT := elFactory(map[interface{}]Element{
		"c": T,
	})
	abT := elFactory(map[interface{}]Element{
		"a": T,
		"b": T,
	})

	tests := []struct {
		a, b      Element
		predicate func(Element) bool
		symbol    string
		expected  bool
	}{
		{liftedMapLat.Bot(), liftedMapLat.Bot(), liftedMapLat.Bot().Eq, "=", true},
		{liftedMapLat.Bot(), liftedMapLat.Lattice.Bot(), liftedMapLat.Bot().Eq, "=", false},
		{liftedMapLat.Bot(), liftedMapLat.Lattice.Bot(), liftedMapLat.Bot().Leq, "⊑", true},
		{liftedMapLat.Bot(), liftedMapLat.Lattice.Bot(), liftedMapLat.Bot().Geq, "⊒", false},
		{liftedMapLat.Top(), liftedMapLat.Top(), liftedMapLat.Top().Eq, "=", true},
		{aT, aT, aT.Leq, "⊑", true},
		{aT, aT, aT.Geq, "⊒", true},
		{aT, bT, aT.Geq, "⊒", false},
		{aT, bT, aT.Leq, "⊑", false},
		{aT, abT, aT.Leq, "⊑", true},
		{bT, abT, bT.Leq, "⊑", true},
		{cT, abT, cT.Leq, "⊑", false},
		{abT, liftedMapLat.Top(), abT.Leq, "⊑", true},
		{abT, liftedMapLat.Top(), abT.Geq, "⊒", false},
		{abT, liftedMapLat.Bot(), abT.Leq, "⊑", false},
		{abT, liftedMapLat.Bot(), abT.Geq, "⊒", true},
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

func TestLiftJoin(t *testing.T) {
	var T = Create().Lattice().TwoElement().Top()

	liftedMapLat := Lift(MakeMapLatticeVariadic[string](Create().Lattice().TwoElement(), "a", "b", "c"))
	elFactory := MakeMap[string](liftedMapLat)
	aT := elFactory(map[interface{}]Element{
		"a": T,
	})
	bT := elFactory(map[interface{}]Element{
		"b": T,
	})
	cT := elFactory(map[interface{}]Element{
		"c": T,
	})
	abT := elFactory(map[interface{}]Element{
		"a": T,
		"b": T,
	})

	tests := []struct {
		a, b, expected Element
	}{
		{liftedMapLat.Bot(), liftedMapLat.Lattice.Bot(), liftedMapLat.Lattice.Bot()},
		{liftedMapLat.Bot(), liftedMapLat.Bot(), liftedMapLat.Bot()},
		{liftedMapLat.Top(), liftedMapLat.Top(), liftedMapLat.Top()},
		{liftedMapLat.Bot(), liftedMapLat.Top(), liftedMapLat.Top()},
		{liftedMapLat.Top(), liftedMapLat.Bot(), liftedMapLat.Top()},
		{liftedMapLat.Bot(), aT, aT},
		{aT, liftedMapLat.Bot(), aT},
		{bT, liftedMapLat.Bot(), bT},
		{cT, liftedMapLat.Bot(), cT},
		{aT, aT, aT},
		{aT, bT, abT},
		{bT, aT, abT},
		{bT, abT, abT},
		{cT, abT, liftedMapLat.Top()},
		{abT, liftedMapLat.Top(), liftedMapLat.Top()},
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
