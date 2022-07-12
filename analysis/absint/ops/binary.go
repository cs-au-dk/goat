package ops

import (
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
	"fmt"
	"go/token"
	"go/types"
	"log"

	"golang.org/x/tools/go/ssa"
)

func BinOp(state L.AnalysisState, v1, v2 L.AbstractValue, ssaVal *ssa.BinOp) (result L.AbstractValue) {
	// Special handling for == and != operators on non-basic types
	switch ssaVal.Op {
	case token.EQL:
		fallthrough
	case token.NEQ:
		// TODO: If only one of the arguments is of interface type, start by
		// dereferencing the interface value and continue as normal.
		xType, yType := ssaVal.X.Type(), ssaVal.Y.Type()
		if _, isBasic := xType.Underlying().(*types.Basic); types.Identical(xType, yType) && !isBasic &&
			!v1.IsWildcard() && !v2.IsWildcard() {

			comparePointers := false
			switch xType.Underlying().(type) {
			case *types.Pointer:
				comparePointers = true
			case *types.Chan:
				comparePointers = true
			case *types.Map:
				comparePointers = true
			case *types.Signature:
				comparePointers = true
			case *types.Slice:
				comparePointers = true
			case *types.Interface:
				return itfBinOp(state, v1.PointerValue(), v2.PointerValue(), ssaVal)
			}

			// Arrays and structs require different handling
			if comparePointers {
				return ptrBinOp(state.Heap(), v1.PointerValue(), v2.PointerValue(), ssaVal.Op)
			}
		}
	}

	switch {
	case !v1.IsBasic() || !v2.IsBasic():
		// If one of the arguments is not a basic type,
		// default to the top value of the type for the receiver SSA value.
		return L.ZeroValueForType(ssaVal.Type()).ToTop()
	case v1.BasicValue().IsBot():
		return v1
	case v2.BasicValue().IsBot():
		return v2
	case v1.BasicValue().IsTop():
		return v1
	case v2.BasicValue().IsTop():
		return v2
	}

	var supported bool
	switch v1 := v1.BasicValue().Value().(type) {
	case int:
		log.Fatalf("A runtime 'int' value appeared here %v %v %T %v %T", ssaVal, v1, v1, v2, v2)

	case int64:
		switch v2 := v2.BasicValue().Value().(type) {
		case int64:
			result, supported = int64BinOp(v1, v2, ssaVal.Op)

		case int:
			log.Fatalf("A runtime 'int' value appeared here %v %v %T %v %T", ssaVal, v1, v1, v2, v2)
		}
	case float64:
		switch v2 := v2.BasicValue().Value().(type) {
		case float64:
			result, supported = float64BinOp(v1, v2, ssaVal.Op)
		}
	case string:
		switch v2 := v2.BasicValue().Value().(type) {
		case string:
			result, supported = stringBinOp(v1, v2, ssaVal.Op)
		}
	case bool:
		switch v2 := v2.BasicValue().Value().(type) {
		case bool:
			result, supported = boolBinOp(v1, v2, ssaVal.Op)
		}
	}

	if supported {
		return result
	}
	return L.ZeroValueForType(ssaVal.Type()).ToTop()
}

func int64BinOp(v1 int64, v2 int64, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res interface{}
	switch op {
	case token.ADD:
		res = v1 + v2
	case token.SUB:
		res = v1 - v2
	case token.MUL:
		res = v1 * v2
	case token.QUO:
		res = v1 / v2
	case token.REM:
		res = v1 % v2
	case token.AND:
		res = v1 & v2
	case token.OR:
		res = v1 | v2
	case token.XOR:
		res = v1 ^ v2
	case token.SHL:
		res = v1 << v2
	case token.SHR:
		res = v1 >> v2
	case token.AND_NOT:
		res = v1 &^ v2
	case token.EQL:
		res = v1 == v2
	case token.NEQ:
		res = v1 != v2
	case token.LSS:
		res = v1 < v2
	case token.LEQ:
		res = v1 <= v2
	case token.GTR:
		res = v1 > v2
	case token.GEQ:
		res = v1 >= v2
	default:
		supported = false
	}
	if res != nil {
		val = L.Elements().AbstractBasic(res)
	}
	return
}

func stringBinOp(v1 string, v2 string, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res interface{}
	switch op {
	case token.ADD:
		res = v1 + v2
	case token.EQL:
		res = v1 == v2
	case token.NEQ:
		res = v1 != v2
	case token.LSS:
		res = v1 < v2
	case token.LEQ:
		res = v1 <= v2
	case token.GTR:
		res = v1 > v2
	case token.GEQ:
		res = v1 >= v2
	default:
		supported = false
	}
	if res != nil {
		val = L.Elements().AbstractBasic(res)
	}
	return
}

func boolBinOp(v1 bool, v2 bool, op token.Token) (val L.AbstractValue, supported bool) {
	var res interface{}
	switch op {
	case token.EQL:
		res = v1 == v2
	case token.NEQ:
		res = v1 != v2
	default:
		supported = false
	}
	if res != nil {
		val = L.Create().Element().AbstractBasic(res)
	}
	return
}

func float64BinOp(v1 float64, v2 float64, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res interface{}
	switch op {
	case token.ADD:
		res = v1 + v2
	case token.SUB:
		res = v1 - v2
	case token.MUL:
		res = v1 * v2
	case token.QUO:
		res = v1 / v2
	case token.EQL:
		res = v1 == v2
	case token.NEQ:
		res = v1 != v2
	case token.LSS:
		res = v1 < v2
	case token.LEQ:
		res = v1 <= v2
	case token.GTR:
		res = v1 > v2
	case token.GEQ:
		res = v1 >= v2
	default:
		supported = false
	}
	if res != nil {
		val = L.Create().Element().AbstractBasic(res)
	}
	return
}

// Utility function for flipping the boolean inside an abstract value
// representing whether two things are equal, if the result should
// represent whether two things are unequal.
func flipEqual(equal L.AbstractValue, op token.Token) L.AbstractValue {
	switch op {
	case token.EQL:
		// Preserve answer
	case token.NEQ:
		// Flip the answer
		if !equal.BasicValue().IsTop() {
			b := equal.BasicValue().Value().(bool)
			equal = L.Create().Element().AbstractBasic(!b)
		}
	default:
		log.Fatalf("Unexpected op for flipEqual: %s", op)
	}

	return equal
}

func ptrBinOp(mem L.Memory, v1, v2 L.PointsTo, op token.Token) L.AbstractValue {
	equal := L.Consts().BotValue()
	intersection := v1.MonoMeet(v2)

	TRUE, FALSE := L.Consts().AbstractBasicBooleans()

	// If the intersection is nonempty, the pointers may be equal
	if !intersection.Empty() {
		equal = TRUE
	}

	mops := L.MemOps(mem)
	representsSingleConcreteLocation := func(v L.PointsTo) bool {
		return v.Eq(L.Consts().PointsToNil()) || mops.CanStrongUpdate(v)
	}

	// If the intersection is empty the pointers are unequal.
	// Pointers can also be unequal if they do not both concretize
	// to a single value.
	if intersection.Empty() || !representsSingleConcreteLocation(v1) ||
		!representsSingleConcreteLocation(v2) {
		equal = equal.MonoJoin(FALSE)
	}

	return flipEqual(equal, op)
}

func itfBinOp(state L.AnalysisState, v1, v2 L.PointsTo, binop *ssa.BinOp) L.AbstractValue {
	// Interface equality specification:
	// Two interface values are equal if they have identical dynamic types and
	// equal dynamic values or if both have value nil.
	equal := L.Consts().BotValue()
	TRUE, FALSE := L.Consts().AbstractBasicBooleans()

	// Fast path for common case.
	v1HasNil, v2HasNil := v1.Contains(loc.NilLocation{}), v2.Contains(loc.NilLocation{})
	if v1HasNil && v2HasNil {
		ptsToNil := L.Consts().PointsToNil()
		// Values may be equal as they can both be nil. They may be unequal if one
		// of the sets contains a non-nil value.
		equal = TRUE
		if !v1.Eq(ptsToNil) || !v2.Eq(ptsToNil) {
			return equal.ToTop()
		}
	} else {
		getMkItf := func(pt loc.Location) *ssa.MakeInterface {
			if pt.Equal(loc.NilLocation{}) {
				return nil
			}

			allocLoc, ok := pt.(loc.AllocationSiteLocation)
			if !ok {
				panic(fmt.Errorf("Expected %v to be an AllocationSiteLocation, was: %T", pt, pt))
			}

			return allocLoc.Site.(*ssa.MakeInterface)
		}

		e1, e2 := v1.Entries(), v2.Entries()
		for _, l1 := range e1 {
			s1 := getMkItf(l1)
			for _, l2 := range e2 {
				if l1.Equal(loc.NilLocation{}) || l2.Equal(loc.NilLocation{}) {
					// If one ptr is nil they are equal iff. they are both nil
					equal = equal.MonoJoin(L.Create().Element().AbstractBasic(l1.Equal(l2)))
				} else {
					s2 := getMkItf(l2)
					a1, a2 := s1.X, s2.X

					if types.Identical(a1.Type(), a2.Type()) {
						fakeBinop := *binop
						fakeBinop.X, fakeBinop.Y, fakeBinop.Op = a1, a2, token.EQL
						equal = equal.MonoJoin(
							// Delegate to BinOp for underlying types
							BinOp(state, state.GetUnsafe(l1), state.GetUnsafe(l2), &fakeBinop),
						)
					} else {
						equal = equal.MonoJoin(FALSE)
					}
				}

				if equal.BasicValue().IsTop() {
					return equal
				}
			}
		}
	}

	return flipEqual(equal, binop.Op)
}
