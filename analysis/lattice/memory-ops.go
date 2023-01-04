package lattice

import (
	"fmt"
	"log"

	loc "github.com/cs-au-dk/goat/analysis/location"
)

// _mops is a wrapper around a memory to facilitate memory operations on arbitrary
// locations, as opposed to only addressable locations.
type _mops struct {
	mem *Memory
}

// MemOps generates a stateful operation-exposing wrapper around the given
// abstract memory.
func MemOps(mem Memory) _mops {
	return _mops{&mem}
}

// CanStrongUpdates checks if strong updates on the given locations in the points-to set
// are allowed. This is the case if the size of the set is 1 and the ALLOC flag is ⊥.
func (m _mops) CanStrongUpdate(set PointsTo) bool {
	if set.Size() != 1 {
		return false
	}

	return !m.IsMultialloc(set.Entries()[0])
}

// IsMultialloc checks whether a given location may have been allocated more than once.
func (m _mops) IsMultialloc(key loc.Location) bool {
	switch key := key.(type) {
	case loc.AddressableLocation:
		return m.mem.IsMultialloc(key)
	case loc.FieldLocation:
		return m.IsMultialloc(key.Base)
	default:
		panic("???")
	}
}

// Update statefully performs updates the binding for the given key with the given value.
// The update is not forced to be weak, but may be depending on circumstances.
func (m _mops) Update(key loc.Location, val AbstractValue) _mops {
	return m.update(key, val, false)
}

// WeakUpdate forcefully weakly updates the binding for the given key with the given value.
func (m _mops) WeakUpdate(key loc.Location, val AbstractValue) _mops {
	return m.update(key, val, true)
}

// UpdateW statefully performs updates the binding for the given key with the given value.
// The update may be forced to be weak with the `forceWeak` flag.
func (m _mops) UpdateW(key loc.Location, val AbstractValue, forceWeak bool) _mops {
	return m.update(key, val, forceWeak)
}

// update performs a maybe-weak update for the given memory.
func (m _mops) update(key loc.Location, val AbstractValue, forceWeak bool) _mops {
	switch key := key.(type) {
	case loc.AddressableLocation:
		if forceWeak {
			if prev, found := m.mem.Get(key); found {
				val = val.MonoJoin(prev)
			}
		}

		*m.mem = m.mem.Update(key, val)
		return m

	case loc.FieldLocation:
		// TODO: This is a bit inefficient since it copies the whole chain of values
		// in each recursive call. We can make the code more convoluted if it
		// becomes a performance issue.
		sval := m.GetUnsafe(key.Base)
		switch {
		case sval.IsStruct():
			fields := sval.StructValue().Update(key.Index, val)
			// Put the updated struct back into the copy of the abstract value
			sval = sval.Update(fields)

			if key.Index == loc.AINDEX {
				// Since we lump all array elements together, we always perform a weak update
				forceWeak = true
			}

		case sval.IsCond():
			sval = sval.UpdateCond(sval.CondValue().UpdateLocker(val.PointsTo()))
		}

		m.update(key.Base, sval, forceWeak)
		return m

	case loc.NilLocation:
		panic(fmt.Errorf("writing %s to nil", val))

	default:
		log.Fatalln("TODO", key)
		panic("")
	}
}

// Get reads arbitrary memory locations, not just addressable locations.
func (m _mops) Get(key loc.Location) (AbstractValue, bool) {
	switch key := key.(type) {
	case loc.AddressableLocation:
		return m.mem.Get(key)

	case loc.FieldLocation:
		sval, found := m.Get(key.Base)
		if !found {
			return sval, false
		}

		switch {
		case sval.IsKnownStruct():
			return sval.StructValue().Get(key.Index).AbstractValue(), true
		case sval.IsCond():
			if !sval.Cond().IsLockerKnown() {
				return Elements().AbstractWildcard(), true
			}
			return Elements().AbstractPointer(
				sval.CondValue().KnownLockers().NonNilEntries(),
			), true
		default:
			panic(fmt.Errorf("FieldLocation %v has unsupported base abstract value: %v", key, sval))
		}
	}

	panic(fmt.Sprintf("Unrecognized abstract location retrieval in memory: %v %T", key, key))
}

// GetUnsafe reads arbitrary memory locations, not just heap locations.
// May panic if the key is not found.
func (m _mops) GetUnsafe(key loc.Location) AbstractValue {
	if val, found := m.Get(key); found {
		return val
	}

	panic(fmt.Sprintf("getUnsafe: %v not found", key))
}

// HeapAlloc is a utility function for allocating values in the heap.
// It properly handles the ALLOC flag of values, indicating whether allocation site has been encountered multiple times.
// If a value already exists at the location it is joined with the supplied value.
func (m _mops) HeapAlloc(loc loc.AllocationSiteLocation, initVal AbstractValue) AbstractValue {
	*m.mem = m.mem.Allocate(loc, initVal, false)
	return Create().Element().AbstractPointerV(loc)
}

func (m _mops) Memory() Memory {
	return *m.mem
}

func (m Memory) LocsToTop(locs ...loc.Location) Memory {
	mops := MemOps(m)

	for _, l := range locs {
		v, ok := mops.Get(l)
		if ok {
			mops.Update(l, v.ToTop())
		}
	}

	return mops.Memory()
}
