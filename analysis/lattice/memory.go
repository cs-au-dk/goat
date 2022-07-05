package lattice

import (
	loc "Goat/analysis/location"
	"Goat/utils"
	i "Goat/utils/indenter"
	"Goat/utils/tree"
	"fmt"
	"log"
	"sort"
)

// TODO: Update memory lattice?
type MemoryLattice struct {
	mapLatticeBase
}

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

func (m *MemoryLattice) Top() Element {
	panic(errUnsupportedOperation)
}

func (m *MemoryLattice) Memory() *MemoryLattice {
	return m
}

var memoryLattice = &MemoryLattice{mapLatticeBase{rng: valueLattice}}

func (latticeFactory) Memory() *MemoryLattice {
	return memoryLattice
}

type addressableLocationHasher loc.LocationHasher

func (addressableLocationHasher) Hash(l loc.AddressableLocation) uint32 {
	return l.Hash()
}
func (addressableLocationHasher) Equal(a, b loc.AddressableLocation) bool {
	return a.Equal(b)
}

type allocationSiteLocationHasher loc.LocationHasher

func (allocationSiteLocationHasher) Hash(l loc.AllocationSiteLocation) uint32 {
	return l.Hash()
}
func (s allocationSiteLocationHasher) Equal(a, b loc.AllocationSiteLocation) bool {
	return a.Equal(b)
}

func (m *MemoryLattice) Bot() Element {
	el := element{lattice: memoryLattice}
	return Memory{
		element: el,
		allocs: tree.NewTree[loc.AllocationSiteLocation, twoElementLatticeElement](
			allocationSiteLocationHasher{}),
		values: tree.NewTree[loc.AddressableLocation, AbstractValue](addressableLocationHasher{}),
	}
}

/* Lattice boilerplate */
func (m *MemoryLattice) String() string {
	return colorize.Lattice("Memory")
}

func (elementFactory) Memory() Memory {
	return memoryLattice.Bot().Memory()
}

type Memory struct {
	element
	// Indicates whether the allocation site has been allocated once (bot) or more (top).
	// Important for strong updates and channel synchronizations.
	allocs tree.Tree[loc.AllocationSiteLocation, twoElementLatticeElement]
	values tree.Tree[loc.AddressableLocation, AbstractValue]
}

// Inserts the key value mapping into the tree, preserving the internal tree
// structure when the mapping already exists.
// Also properly handles weak updates for allocation sites that have ALLOC = ⊤.
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

// Update memory, preventing the insertion of keys which are already
// represented by a top location. Also provides special handling of top
// locations. Provide the key, value, and a fallback memory update function
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

func (w Memory) GetOrDefault(key loc.AddressableLocation, dflt AbstractValue) AbstractValue {
	if v, found := w.Get(key); found {
		return v
	}
	return dflt
}

func (w Memory) GetUnsafe(key loc.AddressableLocation) AbstractValue {
	if v, found := w.Get(key); found {
		return v
	}

	log.Fatalf("GetUnsafe: %s not found", key)
	panic("Unreachable")
}

func (w Memory) IsMultialloc(key loc.AddressableLocation) bool {
	if l, isAllocSite := key.(loc.AllocationSiteLocation); !isAllocSite {
		return false
	} else {
		flag, found := w.allocs.Lookup(l)
		return found && bool(flag)
	}
}

func (w Memory) Update(key loc.AddressableLocation, value AbstractValue) Memory {
	// Ensure that the update is over-approximates soundly in the presence of
	// top locations
	return w.updateTopPreserving(key, value, func() Memory {
		w.values = w.internalInsert(key, value)
		return w
	})
}

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

func (w Memory) ForEach(f func(loc.AddressableLocation, AbstractValue)) {
	w.values.ForEach(f)
}

func (w Memory) Remove(key loc.AddressableLocation) Memory {
	w.values = w.values.Remove(key)

	if aloc, isAllocSite := key.(loc.AllocationSiteLocation); isAllocSite {
		w.allocs = w.allocs.Remove(aloc)
	}
	return w
}

/*
func (w Memory) Find(f func(loc.AddressableLocation, AbstractValue) bool) (zk loc.AddressableLocation, zv AbstractValue, b bool) {
	k, e, found := w.base.Find(func(k interface{}, e Element) bool {
		return f(k.(loc.AddressableLocation), e.(AbstractValue))
	})
	if found {
		return k.(loc.AddressableLocation), e.(AbstractValue), true
	}
	return zk, zv, b
}
*/

// Lattice element methods
func (w Memory) Leq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊑")
	return w.leq(e)
}

func (w Memory) leq(e Element) bool {
	// a ⊑ b ⇔ a ⊔ b == b
	return w.MonoJoin(e.(Memory)).eq(e)
}

func (w Memory) Geq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "⊒")
	return w.geq(e)
}

func (w Memory) geq(e Element) bool {
	// OBS: a ⊑ b ⇔ b ⊒ a
	return e.(Memory).leq(w)
}

func (w Memory) Eq(e Element) bool {
	checkLatticeMatch(w.lattice, e.Lattice(), "=")
	return w.eq(e)
}

func (w Memory) eq(e Element) bool {
	o := e.(Memory)
	return w.values.Equal(o.values, func(a, b AbstractValue) bool {
		return a.Eq(b)
	}) && w.allocs.Equal(o.allocs, func(a, b twoElementLatticeElement) bool {
		return a.Eq(b)
	})
}

func (w Memory) Join(o Element) Element {
	checkLatticeMatch(w.lattice, o.Lattice(), "⊔")
	return w.join(o)
}

func (w Memory) join(o Element) Element {
	return w.MonoJoin(o.(Memory))
}

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

func (w Memory) Meet(o Element) Element {
	panic(errUnsupportedOperation)
}

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

// Type conversion
func (w Memory) Memory() Memory {
	return w
}

// Updates memory to have a list of locations bound to top values
func (mem Memory) InjectTopValues(locs ...loc.AddressableLocation) Memory {
	for _, l := range locs {
		mem.values = mem.internalInsert(l, TopValueForType(l.Type()))
	}

	return mem
}

func (mem Memory) Filter(pred func(loc.AddressableLocation, AbstractValue) bool) Memory {
	fresh := Elements().Memory()

	mem.ForEach(func(al loc.AddressableLocation, av AbstractValue) {
		if pred(al, av) {
			fresh = fresh.Update(al, av)
		}
	})

	return fresh
}

func (mem Memory) Channels() Memory {
	return mem.Filter(func(al loc.AddressableLocation, av AbstractValue) bool {
		return av.IsChan()
	})
}

func (mem Memory) ForChannels(C utils.SSAValueSet) Memory {
	return mem.Filter(func(al loc.AddressableLocation, av AbstractValue) bool {
		s, ok := al.GetSite()
		return av.IsChan() && ok && C.Contains(s)
	})
}
