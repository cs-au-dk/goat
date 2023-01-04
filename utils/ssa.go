package utils

import (
	"fmt"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/pointer"

	"golang.org/x/tools/go/ssa"
)

// FindSSAInstruction returns the first instruction in block-instruction order that matches the predicate.
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

// ValInPkg checks whether an SSA value is included in a given package.
// If the value is a function, it checks the name of the package.
// If the value is not a function, it checks the name of the package of its parent function.
func ValInPkg(val ssa.Value, pkg string) bool {
	switch val := val.(type) {
	case *ssa.Function:
		return val.Pkg.Pkg.Name() == pkg
	default:
		return val.Parent().Pkg.Pkg.Name() == pkg
	}
}

// IsNamedTypeStrict checks whether the given type is the specifed named type.
// Specifically, it checks that the type is a non-aliased named type, and that its package
// name matches the "pkg" parameter, and its name matches the "name" parameter.
func IsNamedTypeStrict(typ types.Type, pkg, name string) bool {
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

// IsNamedType returns whether the given type is the specifed named type or the type of
// pointer to the given named type (not recursively).
func IsNamedType(typ types.Type, pkg string, name string) bool {
	switch typ := typ.(type) {
	case *types.Named:
		return IsNamedTypeStrict(typ, pkg, name)
	case *types.Pointer:
		return IsNamedTypeStrict(typ.Elem(), pkg, name)
	}
	return false
}

// IsModelledConcurrentAPIType checks that a named type is one of
// sync.Mutex, sync.RWMutex or sync.Cond, optionally wrapped in a pointer.
func IsModelledConcurrentAPIType(typ types.Type) bool {
	ptr, ok := typ.(*types.Pointer)
	return IsModelledConcurrentAPITypeStrict(typ) ||
		(ok && IsModelledConcurrentAPITypeStrict(ptr.Elem()))
}

// IsModelledConcurrentAPITypeStrict checks that a named type is strictly one of
// sync.Mutex, sync.RWMutex or sync.Cond, not wrapped in a pointer.
func IsModelledConcurrentAPITypeStrict(typ types.Type) bool {
	return IsNamedTypeStrict(typ, "sync", "Mutex") ||
		IsNamedTypeStrict(typ, "sync", "RWMutex") ||
		IsNamedTypeStrict(typ, "sync", "Cond") ||
		IsNamedTypeStrict(typ, "sync", "WaitGroup")
}

// TypeEmbedsConcurrencyPrimitive checks whether the allocation of an object
// of type embeds by value at least one concurrency primitive.
// This is axiomatically true for standard library concurrency primitive types,
// and extends to types embedding concurrency primitive types without indirection.
func TypeEmbedsConcurrencyPrimitive(typ types.Type) bool {
	switch {
	case IsModelledConcurrentAPITypeStrict(typ):
		// Any standard library concurrency primitive type "embeds" itself.
		return true
	case IsNamedTypeStrict(typ, "sync", "Once"):
		// TODO: The sync.Once struct embeds a mutex. We don't want to treat
		// that as a mutex primitive.
		return false
	}

	switch typ := typ.Underlying().(type) {
	case *types.Struct:
		// For struct types, recursively check each field.
		for i := 0; i < typ.NumFields(); i++ {
			if TypeEmbedsConcurrencyPrimitive(typ.Field(i).Type()) {
				return true
			}
		}
	case *types.Array:
		// For array types, recursively check the element type.
		return TypeEmbedsConcurrencyPrimitive(typ.Elem())
	}

	// Otherwise, the type does not embed concurrency primitives.
	return false
}

func AllocatesConcurrencyPrimitive(v ssa.Value) bool {
	switch v := v.(type) {
	case *ssa.MakeChan:
		return true
	case *ssa.Alloc:
		return TypeEmbedsConcurrencyPrimitive(v.Type().Underlying().(*types.Pointer).Elem())
	case *ssa.MakeSlice:
		return TypeEmbedsConcurrencyPrimitive(v.Type().Underlying().(*types.Slice).Elem())
	default:
		return false
	}
}

// ValHasConcurrencyPrimitive checks whether a value may point to a concurrency primitive,
// given the points-to information.
func ValHasConcurrencyPrimitives(v ssa.Value, pt *pointer.Result) bool {
	// If the underlying type is an interface, then check
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

// TypeHasPointerLikes checks whether a given type is pointer-like,
// or is composed of pointer-like types.
func TypeHasPointerLikes(typ types.Type) bool {
	switch typ := typ.(type) {
	case *types.Array,
		*types.Chan,
		*types.Interface,
		*types.Map,
		*types.Pointer,
		*types.Slice:
		// Arrays, channels, interfaces, maps, pointers and slices
		// are pointer-like by definition.
		return true
	case *types.Named:
		// A named type is pointer-like, if its underlying type is pointer-like.
		return TypeHasPointerLikes(typ.Underlying())
	// Functions are not considered pointer-like because
	// the only relevance is w.r.t the control flow graph.
	// TODO: Flow-sensitivity when taking function successors can help
	// cut down on function successor steppings.
	case *types.Signature:
		// Functions are pointer-like, but we treat them differently.
		return false
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
	case IsNamedType(typ, "sync", "Cond"),
		IsNamedType(typ, "sync", "Mutex"),
		IsNamedType(typ, "sync", "Locker"),
		IsNamedType(typ, "sync", "RWMutex"),
		IsNamedType(typ, "sync", "WaitGroup"):
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
