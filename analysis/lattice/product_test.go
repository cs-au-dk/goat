package lattice

import (
	"testing"
)

func TestProductComparison(t *testing.T) {
	var T = Create().Lattice().TwoElement().Top()
	powLat := Create().Lattice().PowersetVariadic("a", "b")
	setFactory := Create().Element().Powerset(powLat)
	mapLat := Create().Lattice().MapVariadic(Create().Lattice().TwoElement(), "a", "b")
	mapFactory := Create().Element().Map(mapLat)
	prodLat := Create().Lattice().Product(mapLat, powLat)
	prodFactory := Create().Element().Product(prodLat)

	a := setFactory(set{"a": true})
	b := setFactory(set{"b": true})
	aT := mapFactory(map[interface{}]Element{
		"a": T,
	})
	bT := mapFactory(map[interface{}]Element{
		"b": T,
	})

	aTa := prodFactory(aT, a)
	bTa := prodFactory(bT, a)
	aTb := prodFactory(aT, b)
	bTb := prodFactory(bT, b)
	abTa := prodFactory(mapLat.Top(), a)
	abTb := prodFactory(mapLat.Top(), b)
	aTab := prodFactory(aT, powLat.Top())
	bTab := prodFactory(bT, powLat.Top())

	tests := []struct {
		a, b      Element
		predicate func(Element) bool
		symbol    string
		expected  bool
	}{
		{prodLat.Bot(), prodLat.Bot(), prodLat.Bot().Eq, "=", true},
		{prodLat.Top(), prodLat.Top(), prodLat.Top().Eq, "=", true},
		{aTb, aTa, aTb.Geq, "⊒", false},
		{aTa, bTa, aTa.Leq, "⊑", false},
		{aTa, bTb, aTa.Geq, "⊒", false},
		{aTa, bTb, aTa.Leq, "⊑", false},
		{aTa, abTa, aTa.Leq, "⊑", true},
		{abTa, aTa, abTa.Geq, "⊒", true},
		{abTa, prodLat.Top(), abTa.Leq, "⊑", true},
		{abTa, prodLat.Top(), abTa.Geq, "⊒", false},
		{aTab, prodLat.Top(), abTa.Leq, "⊑", true},
		{aTab, prodLat.Top(), abTa.Geq, "⊒", false},
		{bTab, prodLat.Top(), bTab.Leq, "⊑", true},
		{bTab, prodLat.Top(), bTab.Geq, "⊒", false},
		{abTb, prodLat.Top(), abTb.Leq, "⊑", true},
		{abTb, prodLat.Top(), abTb.Geq, "⊒", false},
		{prodLat.Top(), abTa, prodLat.Top().Leq, "⊑", false},
		{prodLat.Top(), abTa, prodLat.Top().Geq, "⊒", true},
		{prodLat.Top(), aTab, prodLat.Top().Leq, "⊑", false},
		{prodLat.Top(), aTab, prodLat.Top().Geq, "⊒", true},
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

func TestProductJoin(t *testing.T) {
	var T = Create().Lattice().TwoElement().Top()
	powLat := Create().Lattice().PowersetVariadic("a", "b")
	setFactory := Create().Element().Powerset(powLat)
	mapLat := Create().Lattice().MapVariadic(Create().Lattice().TwoElement(), "a", "b")
	mapFactory := Create().Element().Map(mapLat)
	prodLat := Create().Lattice().Product(mapLat, powLat)
	prodFactory := Create().Element().Product(prodLat)

	a := setFactory(set{"a": true})
	b := setFactory(set{"b": true})
	aT := mapFactory(map[interface{}]Element{"a": T})
	bT := mapFactory(map[interface{}]Element{"b": T})

	aTa := prodFactory(aT, a)
	bTa := prodFactory(bT, a)
	aTb := prodFactory(aT, b)
	bTb := prodFactory(bT, b)
	abTa := prodFactory(mapLat.Top(), a)
	abTb := prodFactory(mapLat.Top(), b)
	aTab := prodFactory(aT, powLat.Top())
	bTab := prodFactory(bT, powLat.Top())

	tests := []struct {
		a, b, expected Element
	}{
		{prodLat.Bot(), prodLat.Bot(), prodLat.Bot()},
		{prodLat.Top(), prodLat.Top(), prodLat.Top()},
		{prodLat.Bot(), prodLat.Top(), prodLat.Top()},
		{prodLat.Top(), prodLat.Bot(), prodLat.Top()},
		{prodLat.Bot(), aTa, aTa},
		{aTa, prodLat.Bot(), aTa},
		{aTa, bTa, abTa},
		{aTa, aTb, aTab},
		{bTb, aTb, abTb},
		{bTb, bTa, bTab},
		{aTa, bTb, prodLat.Top()},
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
