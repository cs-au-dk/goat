package lattice

import (
	"fmt"
	"log"
	"sort"

	loc "github.com/cs-au-dk/goat/analysis/location"
	i "github.com/cs-au-dk/goat/utils/indenter"
	"github.com/cs-au-dk/goat/utils/tree"
)

// MemoryLattice represents the lattice of abstract memory.
// Its members are maps binding abstract heap locations to abstract values.
type MemoryLattice struct {
	mapLatticeBase
}

// Eq checks for equality with another lattice.
func (m *MemoryLattice) Eq(o Lattice) bool {
	switch o := o.(type) {
	case *MemoryLattice:
		return true
	case *Lifted:
		return m.Eq(o.Lattice)
	case *Dropped:
		return m.Eq(o.Lattice)
	default:
		return false
	}
}

// Top is unsupported for the memory lattice.
func (m *MemoryLattice) Top() Element {
	panic(errUnsupportedOperation)
}

// Memory can safely convert the memory lattice.s
func (m *MemoryLattice) Memory() *MemoryLattice {
	return m
}

// memoryLattice is a singleton instantiation of the memory lattice.
var memoryLattice = &MemoryLattice{mapLatticeBase{rng: valueLattice}}

// Memory creates the memory lattice.
func (latticeFactory) Memory() *MemoryLattice {
	return memoryLattice
}

// addressableLocationHasher is a hasher for addressable locations,
// which are keys in abstract memory values.
type addressableLocationHasher loc.LocationHasher

// Hash computes a 32-bit hash from a given addressable location.
func (addressableLocationHasher) Hash(l loc.AddressableLocation) uint32 {
	return l.Hash()
}

// Equal compares two addressable location for equality.
func (addressableLocationHasher) Equal(a, b loc.AddressableLocation) bool {
	return a.Equal(b)
}

// allocationSiteLocationHasher is a hasher for allocation site locations,
// which are keys in abstract memory values.
type allocationSiteLocationHasher loc.LocationHasher

// Hash computes a 32-bit hash from a given allocation site.
func (allocationSiteLocationHasher) Hash(l loc.AllocationSiteLocation) uint32 {
	return l.Hash()
}

// Equal two allocation site locations for equality.
func (s allocationSiteLocationHasher) Equal(a, b loc.AllocationSiteLocation) bool {
	return a.Equal(b)
}

// Bot computes the ⊥ memory value.
func (m *MemoryLattice) Bot() Element {
	el := element{lattice: memoryLattice}
	return Memory{
		element: el,
		allocs: tree.NewTree[loc.AllocationSiteLocation, twoElementLatticeElement](
			allocationSiteLocationHasher{}),
		values: tree.NewTree[loc.AddressableLocation, AbstractValue](addressableLocationHasher{}),
	}
}

func (m *MemoryLattice) String() string {
	return colorize.Lattice("Memory")
}

// Memory creates a fresh abstract memory
func (elementFactory) Memory() Memory {
	return memoryLattice.Bot().Memory()
}

// Memory is a member of the abstract memory lattice.
// It contains two abstract heaps, one for allocation sites
// and one for addressable locations.
type Memory struct {
	element
	// Indicates whether the allocation site has been allocated once (bot) or more (top).
	// Important for strong updates and channel synchronizations.
	allocs tree.Tree[loc.AllocationSiteLocation, twoElementLatticeElement]
	values tree.Tree[loc.AddressableLocation, AbstractValue]
}

// internalInsert inserts the key value mapping into the tree, preserving the internal tree
// structure when the mapping already exists.
// Also properly handles weak updates for multi-allocated allocation sites
// i.e., where ALLOC = ⊤.
func (w Memory) internalInsert(
	key loc.AddressableLocation,
	value AbstractValue,
) tree.Tree[loc.AddressableLocation, AbstractValue] {
	return w.values.InsertOrMerge(key, value, func(elem, old AbstractValue) (AbstractValue, bool) {
		if elem.eq(old) {
			return old, true
		} else if w.IsMultialloc(key) {
			return elem.MonoJoin(old), false
		} else {
			return elem, false
		}
	})
}

// updateTopPreserving updates memory, bypreventing the insertion of keys which are already
// represented by a top location. Also provides special handling of top
// locations. Requires the key, value, and a fallback memory update function.
func (m Memory) updateTopPreserving(key loc.AddressableLocation, elem AbstractValue, fallback func() Memory) Memory {
	// NOTE (O): The code below assumes that writes that should be
	// redirected to top locations will have no effect on the memory, as the
	// location should already contain a top value.
	// The checks are in place to validate this assumption.
	if IsTopLocation(key) {
		// If the represented top location already exists, then check that the
		// value that was stored there was higher than whatever we are trying
		// to store (safety precaution).
		if v, found := m.values.Lookup(key); found {
			if !v.geq(elem) {
				panic(fmt.Errorf(
					"A value to be stored at top location\n%v\nturned out to be strictly greater than what was stored there before.\nExpected\n%v\n⊑\n%v",
					key, elem, v,
				))
			}
		} else {
			m.values = m.internalInsert(key, elem)
		}

		return m
	}

	// If the key is an allocation site location, check if it has a representative
	if topRep, hasRep := representative(key); hasRep {
		// If the memory already has a represented top location for that
		// SSA site, do not create a new update and instead join with the value
		// found at the top location
		if topLocVal, hasTopLoc := m.values.Lookup(topRep); hasTopLoc {
			if !topLocVal.geq(elem) {
				panic(fmt.Errorf(
					"A value to be stored at location\n%v\nwith top representative\n%v\nturned out to be strictly greater than what was stored there before.\nExpected\n%v\n⊑\n%v",
					key, topRep, elem, topLocVal,
				))
			}

			// TODO: If we want we can try to remove `key` from the memory, as
			// it will never be looked up at this point.
			return m
		}
	}
	// If no condition above was met, return the fallback memory update
	return fallback()
}

// Get retrieves the abstract value at an addressable location.
func (m Memory) Get(key loc.AddressableLocation) (AbstractValue, bool) {
	// First determine whether a top location represents the same allocation site.
	if rep, hasRep := representative(key); hasRep {
		if v, found := m.values.Lookup(rep); found {
			return v, true
		}
	}

	if v, found := m.values.Lookup(key); found {
		return v, found
	}
	return Consts().BotValue(), false
}

// GetOrDefault retrieves the abstract value at an addressable location, or a given
// default value if not found.
func (w Memory) GetOrDefault(key loc.AddressableLocation, dflt AbstractValue) AbstractValue {
	if v, found := w.Get(key); found {
		return v
	}
	return dflt
}

// GetUnsafe retrieves the abstract value at an addressable location.
// Will throw a fatal exception if the location was not found.
func (w Memory) GetUnsafe(key loc.AddressableLocation) AbstractValue {
	if v, found := w.Get(key); found {
		return v
	}

	log.Fatalf("GetUnsafe: %s not found", key)
	panic("Unreachable")
}

// IsMultialloc checks whether a given allocation site was allocated
// multiple times. Non-allocation site addressable locations are not
// multi-allocated by default.
func (w Memory) IsMultialloc(key loc.AddressableLocation) bool {
	if l, isAllocSite := key.(loc.AllocationSiteLocation); !isAllocSite {
		return false
	} else {
		flag, found := w.allocs.Lookup(l)
		return found && bool(flag)
	}
}

// Update takes memory `w` and key `l` and value `v` and returns `w[l ↦ v]`.
func (w Memory) Update(key loc.AddressableLocation, value AbstractValue) Memory {
	// Ensure that the update is over-approximates soundly in the presence of
	// top locations
	return w.updateTopPreserving(key, value, func() Memory {
		w.values = w.internalInsert(key, value)
		return w
	})
}

// Allocate returns the memory updated at the given allocation site as key with the abstract value.
func (w Memory) Allocate(key loc.AllocationSiteLocation, value AbstractValue, forceMultialloc bool) Memory {
	return w.updateTopPreserving(key, value, func() Memory {
		prevFlag, found := w.allocs.Lookup(key)
		if !found {
			w.allocs = w.allocs.Insert(key, twoElementLatticeElement(forceMultialloc))
		} else if !bool(prevFlag) {
			w.allocs = w.allocs.Insert(key, true)
		}

		w.values = w.internalInsert(key, value)
		return w
	})
}

// ForEach executes the given procedure over all addressable locations.
func (w Memory) ForEach(f func(loc.AddressableLocation, AbstractValue)) {
	w.values.ForEach(f)
}

// Remove computes the abstract memory where the given key has been unbound.
func (w Memory) Remove(key loc.AddressableLocation) Memory {
	w.values = w.values.Remove(key)

	if aloc, isAllocSite := key.(loc.AllocationSiteLocation); isAllocSite {
		w.allocs = w.allocs.Remove(aloc)
	}
	return w
}

// Leq computes m ⊑ o. Performs lattice dynamic type checking.
func (w Memory) Leq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊑")
	return w.leq(e)
}

// leq computes m ⊑ o.
func (w Memory) leq(e Element) bool {
	// a ⊑ b ⇔ a ⊔ b == b
	return w.MonoJoin(e.(Memory)).eq(e)
}

// Geq computes m ⊒ o. Performs lattice dynamic type checking.
func (w Memory) Geq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊒")
	return w.geq(e)
}

// geq computes m ⊒ o.
func (w Memory) geq(e Element) bool {
	// OBS: a ⊑ b ⇔ b ⊒ a
	return e.(Memory).leq(w)
}

// Eq computes m = o. Performs lattice dynamic type checking.
func (w Memory) Eq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "=")
	return w.eq(e)
}

// eq computes m = o.
func (w Memory) eq(e Element) bool {
	o := e.(Memory)
	return w.values.Equal(o.values, func(a, b AbstractValue) bool {
		return a.Eq(b)
	}) && w.allocs.Equal(o.allocs, func(a, b twoElementLatticeElement) bool {
		return a.Eq(b)
	})
}

// Join computes m ⊔ o. Performs lattice dynamic type checking.
func (w Memory) Join(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊔")
	return w.join(o)
}

// join computes m ⊔ o.
func (w Memory) join(o Element) Element {
	return w.MonoJoin(o.(Memory))
}

// MonoJoin is a monomorphic variant of m ⊔ o for abstract memory.
func (w Memory) MonoJoin(o Memory) Memory {
	w.values = w.values.Merge(o.values, func(av, bv AbstractValue) (AbstractValue, bool) {
		if av.eq(bv) {
			return av, true
		} else {
			return av.MonoJoin(bv), false
		}
	})
	w.allocs = w.allocs.Merge(o.allocs, func(a, b twoElementLatticeElement) (twoElementLatticeElement, bool) {
		return a.join(b).TwoElement(), a.eq(b)
	})
	return w
}

// Meet computes m ⊓ o. Performs lattice dynamic type checking.
func (w Memory) Meet(o Element) Element {
	panic(errUnsupportedOperation)
}

// meet computes m ⊓ o.
func (w Memory) meet(o Element) Element {
	panic(errUnsupportedOperation)
}

func (w Memory) String() string {
	buf := []string{}

	w.values.ForEach(func(k loc.AddressableLocation, v AbstractValue) {
		if _, isGlobal := k.(loc.GlobalLocation); !isGlobal {
			multiStr := ""
			if w.IsMultialloc(k) {
				multiStr = colorize.Attr("*")
			}
			buf = append(buf, fmt.Sprintf("%s%v ↦ %v", multiStr, k, v))
		}
	})

	sort.Slice(buf, func(i, j int) bool {
		return buf[i] < buf[j]
	})

	return colorize.Field("Memory") + ": " +
		i.Indenter().Start("{").NestStrings(buf...).End("}")
}

// Memory can safely be converted to memory.
func (w Memory) Memory() Memory {
	return w
}

// InjectTopValues updates memory to have a list of locations bound to top values
func (mem Memory) InjectTopValues(locs ...loc.AddressableLocation) Memory {
	for _, l := range locs {
		mem.values = mem.internalInsert(l, TopValueForType(l.Type()))
	}

	return mem
}

// Filter retrieves the abstract memory where only the location-abstract value
// pairs satisfying the given predicate are included.
func (mem Memory) Filter(pred func(loc.AddressableLocation, AbstractValue) bool) Memory {
	fresh := Elements().Memory()

	mem.ForEach(func(al loc.AddressableLocation, av AbstractValue) {
		if pred(al, av) {
			fresh = fresh.Update(al, av)
		}
	})

	return fresh
}

// Channels retrieves all the channels in the abstract memory.
func (mem Memory) Channels() Memory {
	return mem.Filter(func(al loc.AddressableLocation, av AbstractValue) bool {
		return av.IsChan()
	})
}
