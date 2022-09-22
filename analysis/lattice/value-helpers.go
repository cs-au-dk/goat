package lattice

import (
	"fmt"
	T "go/types"
	"log"
	"strings"

	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

// Indexes for value types in product type.
const (
	_POINTER_VALUE = iota
	_CHAN_VALUE
	_STRUCT_VALUE
	_BASIC_VALUE
	_MUTEX_VALUE
	_RWMUTEX_VALUE
	_COND_VALUE
	_WILDCARD_VALUE
	// Untyped abstract value. Only ⊥ and ⊤ are valid untyped values.
	_UNTYPED
)

// Memoize types to avoid needless zero value re-computation.
// NOTE: Types are not canonicalized, so we may end up computing the value for
// the same type multiple times. Consider using typeutil.Map which
// canonicalizes types on insertion.
var botTypeTable = make(map[T.Type]AbstractValue)
var topTypeTable = make(map[T.Type]AbstractValue)

var nilSet = elFact.AbstractPointerV(loc.NilLocation{})

// Computes the zero abstract value for a given type.
// Only provides partial coverage for all given types.
func ZeroValueForType(t T.Type) (zero AbstractValue) {
	if zero, ok := botTypeTable[t]; ok {
		return zero
	}

	switch t := t.(type) {
	case *T.Named:
		if !opts.SkipSync() {
			switch {
			// If the used type is sync.Mutex, instantiate an empty mutex points-to set.
			case utils.IsNamedType(t, "sync", "Mutex"):
				zero = elFact.AbstractMutex()
			case utils.IsNamedType(t, "sync", "RWMutex"):
				zero = elFact.AbstractRWMutex()
			case utils.IsNamedType(t, "sync", "Locker"):
				zero = nilSet
			case utils.IsNamedType(t, "sync", "Cond"):
				zero = elFact.AbstractCond()
			default:
				zero = ZeroValueForType(t.Underlying())
			}
		} else {
			zero = ZeroValueForType(t.Underlying())
		}
	case *T.Pointer:
		zero = nilSet
	case *T.Interface:
		zero = nilSet
	case *T.Chan:
		zero = nilSet
	case *T.Slice:
		zero = nilSet
	case *T.Map:
		zero = nilSet
	case *T.Signature:
		zero = nilSet
	case *T.Struct:
		fields := make(map[interface{}]Element)
		for i := 0; i < t.NumFields(); i++ {
			fields[i] = ZeroValueForType(t.Field(i).Type())
		}
		zero = elFact.AbstractStruct(fields)
	case *T.Tuple:
		fields := make(map[interface{}]Element)
		for i := 0; i < t.Len(); i++ {
			fields[i] = ZeroValueForType(t.At(i).Type())
		}
		zero = elFact.AbstractStruct(fields)
	case *T.Basic:
		switch t.Kind() {
		case T.UnsafePointer:
			zero = nilSet
		case T.Bool:
			zero = elFact.AbstractBasic(false)
		case T.Int, T.Int8, T.Int16, T.Int32, T.Int64,
			T.Uint, T.Uint8, T.Uint16, T.Uint32, T.Uint64:
			// NOTE: Our integer constant propagation works with runtime int64
			// values, so we want all constant integers in the abstract
			// interpreter to be represented by values of that type. This is
			// not sound, but it is a problem for another time.
			zero = elFact.AbstractBasic(int64(0))
		case T.Float32, T.Float64:
			// This is the same.
			zero = elFact.AbstractBasic(float64(0.0))
		case T.String:
			zero = elFact.AbstractBasic("")
		default:
			zero = elFact.AbstractBasic(struct{}{})
		}
	case *T.Array:
		// The current abstraction of arrays lumps all elements together in a single value.
		zero = Elements().AbstractArray(
			ZeroValueForType(t.Elem()))
	default:
		panic(fmt.Errorf("zero value for type %T %v not implemented", t, t))
	}

	botTypeTable[t] = zero
	return zero
}

// Compute the top abstract value for a given type
func TopValueForType(t T.Type) (top AbstractValue) {
	if top, ok := topTypeTable[t]; ok {
		return top
	}

	switch {
	case utils.IsNamedType(t, "sync", "Mutex"):
		if _, ok := t.Underlying().(*T.Struct); ok {
			top = Elements().AbstractMutex().ToTop()
			goto DONE
		}
	case utils.IsNamedType(t, "sync", "RWMutex"):
		if _, ok := t.Underlying().(*T.Struct); ok {
			top = Elements().AbstractRWMutex().ToTop()
			goto DONE
		}
	case utils.IsNamedType(t, "sync", "Cond"):
		if _, ok := t.Underlying().(*T.Struct); ok {
			top = Elements().AbstractCond().ToTop()
			goto DONE
		}
	}

	switch t := t.Underlying().(type) {
	case *T.Array:
		top = Elements().AbstractArray(TopValueForType(t.Elem()))
	case *T.Basic:
		if t.Kind() == T.UnsafePointer {
			top = Consts().WildcardValue()
		} else {
			top = Consts().BasicTopValue()
		}
	case *T.Chan:
		top = Consts().WildcardValue()
	case *T.Interface:
		top = Consts().WildcardValue()
	case *T.Map:
		top = Consts().WildcardValue()
	case *T.Pointer:
		top = Consts().WildcardValue()
	case *T.Signature:
		top = Consts().WildcardValue()
	case *T.Slice:
		top = Consts().WildcardValue()

	case *T.Struct:
		strukt := Elements().AbstractStructV().StructValue()
		for i := 0; i < t.NumFields(); i++ {
			strukt = strukt.Update(i, TopValueForType(t.Field(i).Type()))
		}

		top = Elements().AbstractStructV().Update(strukt)
	case *T.Tuple:
		strukt := Elements().AbstractStructV().StructValue()
		for i := 0; i < t.Len(); i++ {
			strukt = strukt.Update(i, TopValueForType(t.At(i).Type()))
		}

		top = Elements().AbstractStructV().Update(strukt)
	default:
		panic(fmt.Sprintf("Don't know how to inject top for type %s", t))
	}

DONE:
	topTypeTable[t] = top

	return top
}

// Checks whether t1 and t2 are equal. If either t1 or t2 is untyped,
// then return the other t2 or t1, respectively.
// If both types are not untyped and not equal, panic with an unsupported
// type conversion error.
func typeCheckValues(t1, t2 int) int {
	switch {
	// If t1 is _UNTYPED, then
	case t1 == _UNTYPED:
		return t2
	case t2 == _UNTYPED:
		return t1
	case t1 == _WILDCARD_VALUE && t2 == _POINTER_VALUE:
		return t1
	case t1 == _POINTER_VALUE && t2 == _WILDCARD_VALUE:
		return t2
	case t1 != t2:
		avTypeError(t1, t2)
		panic(errUnsupportedTypeConversion)
	default:
		return t1
	}
}

func typeCheckValuesEqual(t1, t2 int) {
	if t1 != t2 {
		avTypeError(t1, t2)
		panic(errUnsupportedTypeConversion)
	}
}

func avTypeError(t1, t2 int) {
	fmt.Printf("\nElements are of type:\n%s\n%s\n", valueLattice.Get(t1), valueLattice.Get(t2))
}

// Configuration structure for creating abstract values
type AbstractValueConfig struct {
	// Constant value (wrapped in interface)
	Basic interface{}
	// Reference to location array. The reference
	// helps distinguish between no PointsTo information
	// (a nil pointer), and the empty pointer location
	// (a nil slice stored at PointsTo)
	PointsTo *[]loc.Location
	// Struct fields
	Struct map[interface{}]Element
	// Abstract value is one of the following.
	// Mutually eclusive.
	Channel  bool
	Mutex    bool
	RWMutex  bool
	Cond     bool
	Wildcard bool
}

func (config AbstractValueConfig) String() string {
	strs := make([]string, 0, 3+len(*config.PointsTo)+len(config.Struct))
	switch {
	case config.Basic != nil:
		strs = append(strs, fmt.Sprintf("Basic: %s", config.Basic))
	case config.PointsTo != nil:
		strs = append(strs, "Points-to:")
		for _, loc := range *config.PointsTo {
			strs = append(strs, loc.String())
		}
	case config.Mutex:
		strs = append(strs, "Is Mutex")
	case config.RWMutex:
		strs = append(strs, "Is RWMutex")
	case config.Channel:
		strs = append(strs, "Is a channel")
	case config.Cond:
		strs = append(strs, "Is Cond")
	case config.Wildcard:
		strs = append(strs, "Is Wildcard")
	case len(config.Struct) != 0:
		strs = append(strs, "Struct fields:")
		for key, val := range config.Struct {
			strs = append(strs, fmt.Sprintf("%v: %s", key, val))
		}
	}

	if len(strs) == 0 {
		return "Empty abstract value configuration!"
	}

	return strings.Join(strs, "\n")
}

func PopulateGlobals(mem Memory, pkgs []*ssa.Package, harnessed bool) Memory {
	for _, pkg := range pkgs {
		for _, member := range pkg.Members {
			if global, ok := member.(*ssa.Global); ok {
				// TODO: We have not yet implemented ZeroValueForType for all types.
				// This means that when a program loads some packages with globals, it will
				// almost certainly panic. This work-around lets the analysis continue, but
				// will fail if the abstract interpreter ever has to use the failing globals.
				func() {
					defer func() {
						if err := recover(); err != nil {
							log.Printf("Global %s: %v", global, err)
						}
					}()

					var v AbstractValue

					// If the function is not harnessed, then interpretation can
					// start with the zero global state.
					if !harnessed {
						v = ZeroValueForType(member.Type().(*T.Pointer).Elem())
					} else {
						// If the function is harnessed, then we have no assumptions
						// about the global state, and must use its top value instead.
						v = TopValueForType(member.Type().(*T.Pointer).Elem())
					}
					mem = mem.Update(loc.GlobalLocation{Site: global}, v)
				}()
			}
		}
	}
	return mem
}
