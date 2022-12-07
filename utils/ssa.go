package utils

import (
	"fmt"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"github.com/benbjohnson/immutable"
	"golang.org/x/tools/go/pointer"

	"golang.org/x/tools/go/ssa"
)

// Returns the first instruction in block-instruction order that matches the predicate.
func FindSSAInstruction(fun *ssa.Function, pred func(ssa.Instruction) bool) (ssa.Instruction, bool) {
	for _, block := range fun.Blocks {
		for _, insn := range block.Instrs {
			if pred(insn) {
				return insn, true
			}
		}
	}
	return nil, false
}

func ValIsInPkg(val ssa.Value, pkg string) bool {
	switch val := val.(type) {
	case *ssa.Function:
		return val.Pkg.Pkg.Name() == pkg
	default:
		return val.Parent().Pkg.Pkg.Name() == pkg
	}
}

func IsNamedType(typ types.Type, pkg string, name string) bool {
	checkNamedType := func(typ types.Type) bool {
		switch typ := typ.(type) {
		case *types.Named:
			if typ.Obj() == nil {
				return false
			}
			if typ.Obj().Pkg() == nil {
				return false
			}
			return !typ.Obj().IsAlias() &&
				typ.Obj().Pkg().Name() == pkg &&
				typ.Obj().Name() == name
		}

		return false
	}

	switch typ := typ.(type) {
	case *types.Named:
		return checkNamedType(typ)
	case *types.Pointer:
		return checkNamedType(typ.Elem())
	}
	return false
}

func IsModelledConcurrentAPIType(typ types.Type) bool {
	return IsNamedType(typ, "sync", "Mutex") ||
		IsNamedType(typ, "sync", "RWMutex") ||
		IsNamedType(typ, "sync", "Cond")
}

func ValHasConcurrencyPrimitives(v ssa.Value, pt *pointer.Result) bool {
	if _, ok := v.Type().Underlying().(*types.Interface); !ok {
		return TypeHasConcurrencyPrimitives(v.Type(), make(map[types.Type]struct{}))
	}

	// If the value is an interface, use the underlying points-to analysis
	// to determine whether it may carry concurrency primitives
	for _, l := range pt.Queries[v].PointsTo().Labels() {
		i, ok := l.Value().(*ssa.MakeInterface)

		if !ok {
			continue
		}

		v := i.X

		// If any of the SSA registers from which
		// an interface is constructed, that may alias
		// this paramater may carry a concurrency primitive
		// then the value is clearly important
		if ValHasConcurrencyPrimitives(v, pt) {
			return true
		}
	}

	return false
}

// func ValMayInvolveConcurrency(pt *pointer.Result, v ssa.Value) bool {
// 	switch t := v.Type() {
// 	}

// }

func TypeHasPointerLikes(typ types.Type) bool {
	switch typ := typ.(type) {
	case *types.Named:
		return TypeHasPointerLikes(typ.Underlying())
	case *types.Array:
		return true
	case *types.Chan:
		return true
	case *types.Interface:
		return true
	case *types.Map:
		return true
	case *types.Pointer:
		return true
	// Functions are not considered pointer-like because
	// the only relevance is w.r.t the control flow graph.
	// TODO: Flow-sensitivity when taking function successors can help
	// cut down on function successor steppings.
	case *types.Signature:
		return false
	case *types.Slice:
		return true
	case *types.Struct:
		for i := 0; i < typ.NumFields(); i++ {
			styp := typ.Field(i).Type()
			if TypeHasPointerLikes(styp) {
				return true
			}
		}
	case *types.Tuple:
		for i := 0; i < typ.Len(); i++ {
			mtyp := typ.At(i).Type()
			if TypeHasPointerLikes(mtyp) {
				return true
			}
		}
	}

	return false
}

func ValHasPointerLikes(v ssa.Value) bool {
	return TypeHasPointerLikes(v.Type())
}

func TypeHasConcurrencyPrimitives(typ types.Type, visited map[types.Type]struct{}) bool {
	switch {
	case IsNamedType(typ, "sync", "Cond"):
		return true
	case IsNamedType(typ, "sync", "Mutex"):
		return true
	case IsNamedType(typ, "sync", "Locker"):
		return true
	case IsNamedType(typ, "sync", "RWMutex"):
		return true
	}

	// TODO: This is not sound for types that contain interfaces that can point to concurrency primitives.
	switch typ := typ.Underlying().(type) {
	case *types.Chan:
		return true
	case *types.Struct:
		for i := 0; i < typ.NumFields(); i++ {
			styp := typ.Field(i).Type()
			if TypeHasConcurrencyPrimitives(styp, visited) {
				return true
			}
		}
	case *types.Tuple:
		for i := 0; i < typ.Len(); i++ {
			mtyp := typ.At(i).Type()
			if TypeHasConcurrencyPrimitives(mtyp, visited) {
				return true
			}
		}
	case *types.Signature:
		if _, seen := visited[typ]; !seen {
			visited[typ] = struct{}{}
			return TypeHasConcurrencyPrimitives(typ.Results(), visited)
		}
	case *types.Array:
		return TypeHasConcurrencyPrimitives(typ.Elem(), visited)
	case *types.Slice:
		if _, seen := visited[typ]; !seen {
			visited[typ] = struct{}{}
			return TypeHasConcurrencyPrimitives(typ.Elem(), visited)
		}
	case *types.Pointer:
		if _, seen := visited[typ]; !seen {
			visited[typ] = struct{}{}
			return TypeHasConcurrencyPrimitives(typ.Elem(), visited)
		}
	}
	return false
}

type SSAValueSet struct {
	*immutable.Map[ssa.Value, struct{}]
}

func (s SSAValueSet) Size() int {
	return s.Map.Len()
}

func MakeSSASet(vs ...ssa.Value) SSAValueSet {
	mp := immutable.NewMap[ssa.Value, struct{}](PointerHasher[ssa.Value]{})
	for _, v := range vs {
		mp = mp.Set(v, struct{}{})
	}

	return SSAValueSet{mp}
}

func (s SSAValueSet) Add(v ssa.Value) SSAValueSet {
	return SSAValueSet{s.Map.Set(v, struct{}{})}
}

func (s1 SSAValueSet) Join(s2 SSAValueSet) SSAValueSet {
	if s1 == s2 {
		return s1
	} else if s2.Size() < s1.Size() {
		s1, s2 = s2, s1
	}

	for iter := s1.Iterator(); !iter.Done(); {
		v, _, _ := iter.Next()
		if !s2.Contains(v) {
			s2.Map = s2.Map.Set(v, struct{}{})
		}
	}

	return s2
}

func (s SSAValueSet) Contains(v ssa.Value) bool {
	_, ok := s.Get(v)
	return ok
}

func (s1 SSAValueSet) Meet(s2 SSAValueSet) SSAValueSet {
	if s1 == s2 {
		return s1
	}

	vs := make([]ssa.Value, 0, s1.Size())

	s1.ForEach(func(v ssa.Value) {
		if s2.Contains(v) {
			vs = append(vs, v)
		}
	})

	return MakeSSASet(vs...)
}

func (s SSAValueSet) ForEach(do func(ssa.Value)) {
	for iter := s.Iterator(); !iter.Done(); {
		next, _, _ := iter.Next()
		do(next)
	}
}

func (s SSAValueSet) Entries() []ssa.Value {
	vs := make([]ssa.Value, 0, s.Size())

	s.ForEach(func(v ssa.Value) {
		vs = append(vs, v)
	})
	return vs
}

func (s SSAValueSet) Empty() bool {
	return s.Map == nil || s.Map.Len() == 0
}

func (s SSAValueSet) String() string {
	vs := s.Entries()

	// Ensure consistent ordering
	sortingKey := func(v ssa.Value) string {
		res := v.Name() + v.String()
		if f := v.Parent(); f != nil {
			res += f.Prog.Fset.Position(v.Pos()).String()
		}
		return res
	}
	sort.Slice(vs, func(i, j int) bool {
		return sortingKey(vs[i]) < sortingKey(vs[j])
	})

	strs := make([]string, s.Size())

	for i, v := range vs {
		str := v.Name() + " = " + v.String()
		if f := v.Parent(); f != nil {
			str += fmt.Sprintf(" at %v (%v)", f.Prog.Fset.Position(v.Pos()), f)
		}
		strs[i] = str
	}

	return "{ " + strings.Join(strs, "\n") + " }"
}

type ssaValueSetHasher struct{}

func (ssaValueSetHasher) Hash(s SSAValueSet) uint32 {
	vs := make([]ssa.Value, 0, s.Size())
	s.ForEach(func(v ssa.Value) {
		vs = append(vs, v)
	})
	// Ensure consistent ordering (NOTE: This only works for singleton P-Sets)
	sortingKey := func(v ssa.Value) string {
		if f := v.Parent(); f != nil {
			prog := f.Prog
			return fmt.Sprintf("%s%s%s", v.Name(), v.String(), prog.Fset.Position(v.Pos()))
		}
		return fmt.Sprintf("%s%s%p", v.Name(), v.String(), v)
	}

	sort.Slice(vs, func(i, j int) bool {
		return sortingKey(vs[i]) < sortingKey(vs[j])
	})

	hashes := make([]uint32, 0, s.Size())
	for _, v := range vs {
		hashes = append(hashes, PointerHasher[ssa.Value]{}.Hash(v))
	}

	return HashCombine(hashes...)
}

func (ssaValueSetHasher) Equal(a, b SSAValueSet) bool {
	if a == b {
		return true
	} else if a.Size() != b.Size() {
		return false
	}

	for it := a.Map.Iterator(); !it.Done(); {
		if k, _, _ := it.Next(); !b.Contains(k) {
			return false
		}
	}
	return true
}

var SSAValueSetHasher immutable.Hasher[SSAValueSet] = ssaValueSetHasher{}

func PrintSSAFun(fun *ssa.Function) {
	fmt.Println(fun.Name())
	for bi, b := range fun.Blocks {
		fmt.Println(bi, ":")
		for _, i := range b.Instrs {
			switch v := i.(type) {
			case *ssa.DebugRef:
				// skip
			case ssa.Value:
				fmt.Println(v.Name(), "=", v)
			default:
				fmt.Println(i)
			}
		}
	}
}

func PrintSSAFunWithPos(fset *token.FileSet, fun *ssa.Function) {
	fmt.Println(fun.Name())
	for bi, b := range fun.Blocks {
		fmt.Println(bi, ":")
		for _, i := range b.Instrs {
			switch v := i.(type) {
			case *ssa.DebugRef:
				// skip
			case ssa.Value:
				fmt.Println(v.Name(), "=", v, "at position:", fset.Position(v.Pos()))
			default:
				fmt.Println(i, "at position:", fset.Position(i.Pos()))
			}
		}
	}
}
