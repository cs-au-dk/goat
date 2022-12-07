package lattice

import "testing"

func TestMapComparison(t *testing.T) {
	var T = Create().Lattice().TwoElement().Top()

	mapLat := MakeMapLatticeVariadic[string](Create().Lattice().TwoElement(), "a", "b", "c")
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
		{mapLat.Bot(), mapLat.Bot(), mapLat.Bot().Eq, "=", true},
		{mapLat.Top(), mapLat.Top(), mapLat.Top().Eq, "=", true},
		{aT, aT, aT.Leq, "⊑", true},
		{aT, aT, aT.Geq, "⊒", true},
		{aT, bT, aT.Geq, "⊒", false},
		{aT, bT, aT.Leq, "⊑", false},
		{aT, abT, aT.Leq, "⊑", true},
		{bT, abT, bT.Leq, "⊑", true},
		{cT, abT, cT.Leq, "⊑", false},
		{abT, mapLat.Top(), abT.Leq, "⊑", true},
		{abT, mapLat.Top(), abT.Geq, "⊒", false},
		{abT, mapLat.Bot(), abT.Leq, "⊑", false},
		{abT, mapLat.Bot(), abT.Geq, "⊒", true},
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

func TestMapJoin(t *testing.T) {
	var T = Create().Lattice().TwoElement().Top()

	mapLat := MakeMapLatticeVariadic[string](Create().Lattice().TwoElement(), "a", "b", "c")
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
		a, b, expected Element
	}{
		{mapLat.Bot(), mapLat.Bot(), mapLat.Bot()},
		{mapLat.Top(), mapLat.Top(), mapLat.Top()},
		{mapLat.Bot(), mapLat.Top(), mapLat.Top()},
		{mapLat.Top(), mapLat.Bot(), mapLat.Top()},
		{mapLat.Bot(), aT, aT},
		{aT, mapLat.Bot(), aT},
		{bT, mapLat.Bot(), bT},
		{cT, mapLat.Bot(), cT},
		{aT, aT, aT},
		{aT, bT, abT},
		{bT, aT, abT},
		{bT, abT, abT},
		{cT, abT, mapLat.Top()},
		{abT, mapLat.Top(), mapLat.Top()},
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

func TestMapUpdate(t *testing.T) {
	var T = Create().Lattice().TwoElement().Top()

	mapLat := MakeMapLatticeVariadic[string](Create().Lattice().TwoElement(), "a", "b", "c")
	elFactory := MakeMap[string](mapLat)
	mp := elFactory(map[interface{}]Element{
		"a": T,
	})
	abT := elFactory(map[interface{}]Element{
		"a": T,
		"b": T,
	})

	tests := []struct {
		a, expected Map[string]
		k           string
		v           Element
	}{
		{mp, abT, "b", T},
	}

	for _, test := range tests {
		res := test.a.Update(test.k, test.v)
		if !res.Eq(test.expected) {
			t.Errorf("%s[ %s ↦ %s ] = %s, expected %s\n", test.a, test.k, test.v, res, test.expected)
		} else {
			t.Logf("%s[ %s ↦ %s ] = %s\n", test.a, test.k, test.v, res)
		}
	}
}
