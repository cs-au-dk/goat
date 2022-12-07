package ops

import (
	"go/token"
	"log"

	L "github.com/cs-au-dk/goat/analysis/lattice"

	"golang.org/x/tools/go/ssa"
)

func UnOp(v L.AbstractValue, ssaVal *ssa.UnOp) (result L.AbstractValue) {
	if !v.IsBasic() {
		panic("what")
	}

	basic := v.BasicValue()
	if basic.IsBot() || basic.IsTop() {
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

	if supported {
		return result
	}

	return L.ZeroValueForType(ssaVal.Type()).ToTop()
}

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
