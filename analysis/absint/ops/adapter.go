package ops

import (
	L "Goat/analysis/lattice"
	T "go/types"
)

// Adapt types for implicit/permissive type coercions allowed by Golang
func TypeAdapter(from, to T.Type, v L.AbstractValue) L.AbstractValue {
	switch from := from.Underlying().(type) {
	case *T.Basic:
		if from.Kind() != T.String {
			break
		}

		switch to := to.Underlying().(type) {
		case *T.Array:
			toe, ok := to.Elem().Underlying().(*T.Basic)
			if ok && toe.Kind() == T.Rune || toe.Kind() == T.Byte {
				return L.Elements().AbstractArray(L.Consts().BasicTopValue())
			}
		case *T.Slice:
			toe, ok := to.Elem().Underlying().(*T.Basic)
			if ok && toe.Kind() == T.Rune || toe.Kind() == T.Byte {
				return L.Elements().AbstractArray(L.Consts().BasicTopValue())
			}
		}
	case *T.Array:
		switch to := to.Underlying().(type) {
		case *T.Basic:
			if to.Kind() != T.String {
				break
			}

			t1e, ok := from.Elem().Underlying().(*T.Basic)
			if ok && t1e.Kind() == T.Rune || t1e.Kind() == T.Byte {
				return L.Consts().BasicTopValue()
			}
		}
	case *T.Slice:
		switch to := to.Underlying().(type) {
		case *T.Basic:
			if to.Kind() != T.String {
				break
			}

			frome, ok := from.Elem().Underlying().(*T.Basic)
			if ok && frome.Kind() == T.Rune || frome.Kind() == T.Byte {
				return L.Consts().BasicTopValue()
			}
		}
	}

	return v
}
