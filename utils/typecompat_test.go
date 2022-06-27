package utils

import (
	"go/token"
	T "go/types"
	"testing"
)

func TestTypeCompat(t *testing.T) {
	type test struct {
		declType, allocType T.Type
		expected            bool
	}

	String := T.Typ[T.String]
	Int := T.Typ[T.Int]
	MyInt := T.NewNamed(
		T.NewTypeName(token.NoPos, nil, "myint", nil),
		Int,
		nil,
	)

	PInt := T.NewPointer(Int)
	PMyInt := T.NewPointer(MyInt)

	// Interface with function f
	fSig := T.NewSignatureType(nil, nil, nil, nil, nil, false)
	fFunc := T.NewFunc(token.NoPos, nil, "f", fSig)

	bareItf := T.NewInterfaceType([]*T.Func{fFunc}, nil)
	bareItf.Complete()

	Itf := T.NewNamed(
		T.NewTypeName(token.NoPos, nil, "Itf", nil),
		bareItf,
		nil,
	)

	// Interface that embeds Itf
	bareEItf := T.NewInterfaceType(nil, []T.Type{Itf})
	bareEItf.Complete()

	EItf := T.NewNamed(
		T.NewTypeName(token.NoPos, nil, "EItf", nil),
		bareEItf,
		nil,
	)

	// Interface that embeds Itf and has a two functions f and g
	gFunc := T.NewFunc(token.NoPos, nil, "g", fSig)

	bareSubItf := T.NewInterfaceType([]*T.Func{fFunc, gFunc}, nil)
	bareSubItf.Complete()

	SubItf := T.NewNamed(
		T.NewTypeName(token.NoPos, nil, "SubItf", nil),
		bareSubItf,
		nil,
	)

	for _, test := range [...]test{
		{Int, Int, true},
		{Int, MyInt, true},
		{MyInt, Int, true},
		{Int, String, false},
		{PInt, PInt, true},
		{PInt, T.NewPointer(Int), true},
		{PInt, PMyInt, true},
		{PMyInt, PInt, true},
		{Itf, Itf, true},
		{Itf, EItf, true},
		{Itf, SubItf, true},
		// Possible with type assertions, see top-pointers/param/interface/typeasserted
		{SubItf, Itf, true},
	} {
		if actual := TypeCompat(test.declType, test.allocType); actual != test.expected {
			t.Errorf("Expected TypeCompat(%v, %v) = %v, was %v",
				test.declType, test.allocType, actual, test.expected)
		}
	}
}
