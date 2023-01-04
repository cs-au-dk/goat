package ops

import (
	"go/token"
	"log"

	L "github.com/cs-au-dk/goat/analysis/lattice"

	"golang.org/x/tools/go/ssa"
)

// UnOp encodes the semantics over abstract values of unary operations over basic types.
func UnOp(v L.AbstractValue, ssaVal *ssa.UnOp) (result L.AbstractValue) {
	if !v.IsBasic() {
		panic("Unary operation for basic values performed on non-basic value.")
	}

	basic := v.BasicValue()
	if basic.IsBot() || basic.IsTop() {
		// Return ⊥ and ⊤ unchanged.
		return v
	}

	var supported bool
	switch val := basic.Value().(type) {
	case int:
		log.Fatalf("A runtime 'int' value appeared here %v %v %T", ssaVal, v, v)
	case int64:
		result, supported = int64UnOp(val, ssaVal.Op)
	case bool:
		result, supported = boolUnOp(val, ssaVal.Op)
	case float64:
		result, supported = float64UnOp(val, ssaVal.Op)
	}

	// If the operation is supported, return the computed result.
	if supported {
		return result
	}
	// Otherwise return the top value for the type of the result.
	return L.ZeroValueForType(ssaVal.Type()).ToTop()
}

// int64UnOp leverages concrete semantics for int64 unary operations which are known
// constant values. It returns the resulting abstract value, and true if the unary operation
// is supported, or ⊥ and false otherwise.
func int64UnOp(v int64, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res int64
	switch op {
	case token.SUB:
		res = -v
	case token.XOR:
		res = ^v
	default:
		supported = false
	}

	return L.Elements().AbstractBasic(res), supported
}

// float64UnOp leverages concrete semantics for float64 unary operations which are known
// constant values. It returns the resulting abstract value, and true if the unary operation
// is supported, or ⊥ and false otherwise.
func float64UnOp(v float64, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res float64
	switch op {
	case token.SUB:
		res = -v
	default:
		supported = false
	}

	return L.Elements().AbstractBasic(res), supported
}

// bool4UnOp leverages concrete semantics for boolean unary operations which are known
// constant values. It returns the resulting abstract value, and true if the unary operation
// is supported, or ⊥ and false otherwise.
func boolUnOp(v bool, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res bool
	switch op {
	case token.NOT:
		res = !v
	default:
		supported = false
	}

	return L.Elements().AbstractBasic(res), supported
}
