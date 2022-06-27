package lattice

import (
	"Goat/utils"
	"go/types"
)

type AbstractValueLattice struct {
	ProductLattice
}

// TODO: Consider using lifted/dropped lattices to signify unused parts
var valueLattice = func() *AbstractValueLattice {
	prod := *latFact.Product(
		// Points to set for pointer values.
		pointsToLattice,
		// Channel information for channel "allocation sites".
		Lift(channelInfoLattice),
		// Struct abstract value lattice. A map infinite in the domain, "Fields"
		// (an alias for constants), ranging over a lifted, dropped infiinite map
		Lift(Drop(latFact.InfiniteMap(nil, "Fields"))),
		// Constant propagation element for basic values.
		latFact.ConstantPropagation(),
		// Mutex information for mutex "allocation sites"
		mutexLattice,
		// RWMutex information for RW mutex "allocation sites"
		rwmutexLattice,
		// Cond information for Cond "allocation sites"
		condLattice,
		// Wildcard component, used for unknown pointer-like values
		Lift(oneElementLattice),
	)

	val := &AbstractValueLattice{prod}
	prod.Get(_STRUCT_VALUE).InfiniteMap().rng = val

	return val
}()

func (latticeFactory) AbstractValue() *AbstractValueLattice {
	return valueLattice
}

func (v *AbstractValueLattice) AbstractValue() *AbstractValueLattice {
	return v
}

func (v *AbstractValueLattice) Top() Element {
	return AbstractValue{
		element{v},
		v.ProductLattice.Top().Product(),
		_UNTYPED,
	}
}

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

func (l *AbstractValueLattice) Product() *ProductLattice {
	return l.ProductLattice.Product()
}

func (l *AbstractValueLattice) Value() *AbstractValueLattice {
	return l
}

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
