package ops

import (
	L "Goat/analysis/lattice"
	"go/token"

	"golang.org/x/tools/go/ssa"
)

func UnOp(v L.AbstractValue, ssaVal *ssa.UnOp) (result L.AbstractValue) {
	switch {
	case !v.IsBasic():
		panic("what")

	}

	basic := v.BasicValue()
	if basic.IsBot() ||
		basic.IsTop() {
		return v
	}

	var supported bool
	switch val := basic.Value().(type) {
	case int:
		result, supported = intUnOp(val, ssaVal.Op)
	case bool:
		result, supported = boolUnOp(val, ssaVal.Op)
	case float64:
		result, supported = float64UnOp(val, ssaVal.Op)
	}

	if supported {
		return result
	}

	return L.ZeroValueForType(ssaVal.Type()).ToTop()
}

func intUnOp(v int, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res int
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
