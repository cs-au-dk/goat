package ops

import (
	T "go/types"

	L "github.com/cs-au-dk/goat/analysis/lattice"
)

// Adapt types for implicit/permissive type coercions allowed by Golang.
// Takes abstract value `v` and converts from value of type `from` to equivalent
// value of type `to`.
func TypeAdapter(from, to T.Type, v L.AbstractValue) L.AbstractValue {
	switch from := from.Underlying().(type) {
	case *T.Basic:
		// For basic types, only strings can be coerced into slices or arrays of bytes or runes.
		if from.Kind() != T.String {
			break
		}

		switch to := to.Underlying().(type) {
		case *T.Array:
			toe, ok := to.Elem().Underlying().(*T.Basic)
			// Convert string value to [n]byte OR [n]rune value
			if ok && toe.Kind() == T.Rune || toe.Kind() == T.Byte {
				// Yields an abstract array value containing the basic ⊤ value in the constant propagation lattice.
				return L.Elements().AbstractArray(L.Consts().BasicTopValue())
			}
		case *T.Slice:
			toe, ok := to.Elem().Underlying().(*T.Basic)
			// Convert string to []byte OR []rune
			if ok && toe.Kind() == T.Rune || toe.Kind() == T.Byte {
				// Yields an abstract array value containing the basic ⊤ value in the constant propagation lattice.
				return L.Elements().AbstractArray(L.Consts().BasicTopValue())
			}
		}
	case *T.Array:
		// Rune or byte arrays can be coerced into strings.
		switch to := to.Underlying().(type) {
		case *T.Basic:
			if to.Kind() != T.String {
				break
			}

			t1e, ok := from.Elem().Underlying().(*T.Basic)
			// Convert [n]byte OR [n]rune value to string value
			if ok && t1e.Kind() == T.Rune || t1e.Kind() == T.Byte {
				// Yields the basic ⊤ value in the constant propagation lattice.
				return L.Consts().BasicTopValue()
			}
		}
	case *T.Slice:
		// Rune or byte slices can be coerced into strings.
		switch to := to.Underlying().(type) {
		case *T.Basic:
			if to.Kind() != T.String {
				break
			}

			frome, ok := from.Elem().Underlying().(*T.Basic)
			// Converts []byte OR []rune value to a string value
			if ok && frome.Kind() == T.Rune || frome.Kind() == T.Byte {
				// Yields the basic ⊤ value in the constant propagation lattice.
				return L.Consts().BasicTopValue()
			}
		}
	}

	// If none of the implicit conversions occurred, the abstract value
	// remains unchanged.
	return v
}
