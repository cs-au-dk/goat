package ops

import (
	"fmt"
	"go/token"
	"go/types"
	"log"

	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"

	"golang.org/x/tools/go/ssa"
)

// BinOp encodes the  semantics of a binary SSA operation over abstract values.
func BinOp(mem L.Memory, v1, v2 L.AbstractValue, ssaVal *ssa.BinOp) (result L.AbstractValue) {
	// Special handling for == and != operators on non-basic types
	switch ssaVal.Op {
	case token.EQL, token.NEQ:
		// TODO: If only one of the arguments is of interface type, start by
		// dereferencing the interface value and continue as normal.
		xType, yType := ssaVal.X.Type(), ssaVal.Y.Type()
		if _, isBasic := xType.Underlying().(*types.Basic); types.Identical(xType, yType) && !isBasic &&
			!v1.IsWildcard() && !v2.IsWildcard() {

			// If neither values is a wildcard or of a basic type,
			// check whether pointer comparison has to be carried out.
			comparePointers := false
			switch xType.Underlying().(type) {
			case *types.Pointer,
				*types.Chan,
				*types.Map,
				*types.Signature,
				*types.Slice:
				// Any of the types above implies pointer comparison
				comparePointers = true
			case *types.Interface:
				// Interface comparison.
				return itfBinOp(mem, v1.PointerValue(), v2.PointerValue(), ssaVal)
			}

			// Arrays and structs require different handling.
			if comparePointers {
				return ptrBinOp(mem, v1.PointerValue(), v2.PointerValue(), ssaVal.Op)
			}
		}
	}

	switch {
	case !v1.IsBasic() || !v2.IsBasic():
		// If one of the arguments is not a basic type,
		// default to the top value for the type of the SSA register
		// storing the result.
		return L.ZeroValueForType(ssaVal.Type()).ToTop()
	case v1.BasicValue().IsBot():
		// If v1 = ⊥, then return v1
		return v1
	case v2.BasicValue().IsBot():
		// If v2 = ⊥, then return v2
		return v2
	case v1.BasicValue().IsTop():
		// If v1 = ⊤, then return v1
		return v1
	case v2.BasicValue().IsTop():
		// If v2 = ⊤, then return v2
		return v2
	}

	var supported bool
	switch v1 := v1.BasicValue().Value().(type) {
	case int:
		log.Fatalf("A runtime 'int' value appeared here %v %v %T %v %T", ssaVal, v1, v1, v2, v2)

	case int64:
		switch v2 := v2.BasicValue().Value().(type) {
		case int64:
			// If both abstract values are known int64 constants, return
			// the result of performing the concrete operation.
			result, supported = int64BinOp(v1, v2, ssaVal.Op)

		case int:
			log.Fatalf("A runtime 'int' value appeared here %v %v %T %v %T", ssaVal, v1, v1, v2, v2)
		}
	case float64:
		switch v2 := v2.BasicValue().Value().(type) {
		case float64:
			// If both abstract values are known float64 constants, return
			// the result of performing the concrete operation.
			result, supported = float64BinOp(v1, v2, ssaVal.Op)
		}
	case string:
		switch v2 := v2.BasicValue().Value().(type) {
		case string:
			// If both abstract values are known strings, return
			// the result of performing the concrete operation.
			result, supported = stringBinOp(v1, v2, ssaVal.Op)
		}
	case bool:
		switch v2 := v2.BasicValue().Value().(type) {
		case bool:
			// If both abstract values are known boolean constants, return
			// the result of performing the concrete operation.
			result, supported = boolBinOp(v1, v2, ssaVal.Op)
		}
	}

	// If the operation is supported, return the computed result.
	if supported {
		return result
	}
	// Otherwise return the top value for the type of the result.
	return L.ZeroValueForType(ssaVal.Type()).ToTop()
}

// int64BinOp leverages concrete semantics for int64 binary operations which are
// known constant values. It returns the resulting abstract value, and true if the binary operation
// is supported, or ⊥ and false otherwise.
func int64BinOp(v1 int64, v2 int64, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res any
	switch op {
	case token.ADD:
		res = v1 + v2
	case token.SUB:
		res = v1 - v2
	case token.MUL:
		res = v1 * v2
	case token.QUO:
		if v2 == 0 {
			// Return ⊥ if guaranteed to divide by 0
			return L.Consts().BotValue(), true
		}

		res = v1 / v2
	case token.REM:
		if v2 == 0 {
			// Return ⊥ if guaranteed to get the remained of division by 0
			return L.Consts().BotValue(), true
		}

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

// stringBinOp leverages concrete semantics for string binary operations which are
// known constant values. It returns the resulting abstract value, and true if the binary operation
// is supported, or ⊥ and false otherwise.
func stringBinOp(v1 string, v2 string, op token.Token) (val L.AbstractValue, supported bool) {
	supported = true
	var res any
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

// boolBinOp leverages concrete semantics for boolean binary operations which are known
// constant values. It returns the resulting abstract value, and true if the binary operation
// is supported, or ⊥ and false otherwise.
func boolBinOp(v1 bool, v2 bool, op token.Token) (val L.AbstractValue, supported bool) {
	var res any
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

// float64BinOp leverages concrete semantics for float64 binary operations which are known
// constant values. It returns the resulting abstract value, and true if the binary operation
// is supported, or ⊥ and false otherwise.
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

// flipEqual is an utility function for flipping the boolean constant encoded by an abstract value.
//
//	flipEqual(x, t) = x, if x ∈ {⊥, ⊤} v t = EQL
//	flipEqual(x, NEQ) = ¬x, if x ∈ {true, false}
func flipEqual(equal L.AbstractValue, op token.Token) L.AbstractValue {
	switch op {
	case token.EQL:
		// Preserve answer
	case token.NEQ:
		// Flip the answer if it is not top.
		if !equal.BasicValue().IsTop() {
			b := equal.BasicValue().Value().(bool)
			equal = L.Create().Element().AbstractBasic(!b)
		}
	default:
		log.Fatalf("Unexpected op for flipEqual: %s", op)
	}

	return equal
}

// ptrBinOp encodes the semantics for pointer binary operations which are known
// constant values. It returns the resulting abstract value, and true if the binary operation
// is supported, or ⊥ and false otherwise.
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

// itfBinOp performs binary operations on two interface points-to values.
// Two interface values may be equal if they have identical dynamic types and
// equal dynamic values or if both contain value nil.
func itfBinOp(mem L.Memory, v1, v2 L.PointsTo, binop *ssa.BinOp) L.AbstractValue {
	// Start with the result as bottom
	equal := L.Consts().BotValue()
	// Unpack abstract boolean constants.
	TRUE, FALSE := L.Consts().AbstractBasicBooleans()

	// Check if both points-to sets contain nil
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
		// Extract make interface instruction from an abstract heap location, or nil for a nil location.
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
		mops := L.MemOps(mem)
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
						// Compare values at the underlying locations in the points-to sets.
						equal = equal.MonoJoin(
							// Delegate to BinOp for underlying types
							BinOp(mem, mops.GetUnsafe(l1), mops.GetUnsafe(l2), &fakeBinop),
						)
					} else {
						equal = equal.MonoJoin(FALSE)
					}
				}

				// If equality value is already top, short-circuit evaluation and return it.
				if equal.BasicValue().IsTop() {
					return equal
				}
			}
		}
	}

	// Return the flipped value of equal (if required).
	return flipEqual(equal, binop.Op)
}
