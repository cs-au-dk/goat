package lattice

import (
	"go/types"

	"github.com/cs-au-dk/goat/utils"
)

// AbstractValueLattice is the lattice of abstract Go values.
type AbstractValueLattice struct {
	ProductLattice
}

// valueLattice is a singleton instantiation of the abstract value lattice.
var valueLattice = func() *AbstractValueLattice {
	prod := *latFact.Product(
		// Points-to set lattice for pointer values.
		pointsToLattice,
		// Channel lattice for information about channels stored on the heap.
		Lift(channelInfoLattice),
		// Struct abstract value lattice. A map infinite in the domain, "Fields"
		// (an alias for constants), ranging over a lifted, dropped infinite map.
		Lift(Drop(MakeInfiniteMapLattice[any](nil, "Fields"))),
		// Constant propagation element for basic values.
		latFact.ConstantPropagation(),
		// Mutex latex for mutex "allocation sites"
		mutexLattice,
		// RWMutex information for RW mutex "allocation sites"
		rwmutexLattice,
		// Cond information for Cond "allocation sites"
		condLattice,
		// WaitGroup information for WaitGroup "allocation sites"
		flatIntLattice,
		// Wildcard component, used for unknown pointer-like values
		Lift(oneElementLattice),
	)

	val := &AbstractValueLattice{prod}
	unwrapLattice[*InfiniteMapLattice[any]](prod.Get(_STRUCT_VALUE)).rng = val

	return val
}()

// AbstractValue produces the abstract value lattice.
func (latticeFactory) AbstractValue() *AbstractValueLattice {
	return valueLattice
}

// AbstractValue safely converts the abstract value lattice.
func (v *AbstractValueLattice) AbstractValue() *AbstractValueLattice {
	return v
}

// Top returns the abstract value ⊤, which is untyped.
func (v *AbstractValueLattice) Top() Element {
	return AbstractValue{
		element{v},
		v.ProductLattice.Top().Product(),
		_UNTYPED,
	}
}

// Bot returns the abstract value ⊥, which is untyped. The raw
// ⊥ is only meant to be used as a baseline for computing the least-upper bound.
func (v *AbstractValueLattice) Bot() Element {
	return AbstractValue{
		element{v},
		v.ProductLattice.Bot().Product(),
		_UNTYPED,
	}
}

func (l1 *AbstractValueLattice) Eq(l2 Lattice) bool {
	switch l2 := l2.(type) {
	case *AbstractValueLattice:
		return true
	case *Lifted:
		return l1.Eq(l2.Lattice)
	case *Dropped:
		return l1.Eq(l2.Lattice)
	}
	return false
}

func (AbstractValueLattice) String() string {
	return colorize.Lattice("Value")
}

func (l *AbstractValueLattice) Get(i int) Lattice {
	return l.ProductLattice.Get(i)
}

// Product converts to underlying product lattice.
func (l *AbstractValueLattice) Product() *ProductLattice {
	return l.ProductLattice.Product()
}

// Value safely converts to abstract value lattice.
func (l *AbstractValueLattice) Value() *AbstractValueLattice {
	return l
}

// LatticeForType retrieves the correspoding lattice of abstract values
// corresponding to an arbitrary Go type.
func (l *AbstractValueLattice) LatticeForType(T types.Type) Lattice {
	switch T := T.(type) {
	case *types.Pointer:
		return l.Get(_POINTER_VALUE)
	case *types.Interface:
		return l.Get(_POINTER_VALUE)
	case *types.Signature:
		return l.Get(_POINTER_VALUE)
	case *types.Chan:
		return l.Get(_CHAN_VALUE)
	case *types.Named:
		switch {
		case utils.IsNamedType(T, "sync", "Mutex"):
			return l.Get(_MUTEX_VALUE)
		case utils.IsNamedType(T, "sync", "RWMutex"):
			return l.Get(_RWMUTEX_VALUE)
		case utils.IsNamedType(T, "sync", "Cond"):
			return l.Get(_COND_VALUE)
		case utils.IsNamedType(T, "sync", "WaitGroup"):
			return l.Get(_WAITGROUP_VALUE)
		default:
			return l.LatticeForType(T.Underlying())
		}
	case *types.Tuple:
		return l.Get(_STRUCT_VALUE)
	case *types.Struct:
		return l.Get(_STRUCT_VALUE)
	case *types.Basic:
		return l.Get(_BASIC_VALUE)
	default:
		panic(errNotImplemented)
		// case *types.
	}
}
