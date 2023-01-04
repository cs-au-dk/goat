package ops

import (
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
)

// ToDeref retrieves all the locations that could be dereferenced
// by a dereferencing operation.
func ToDeref(v L.AbstractValue) L.OpOutcomes {
	OUTCOME, SUCCEEDS, PANICS := L.Consts().OpOutcomes()
	// The points-to set including only the nil location
	PTNIL := L.Consts().PointsToNil()

	pt := v.PointerValue()

	// If the points-to set contains nil, dereferencing may panic.
	if pt.HasNil() {
		OUTCOME = OUTCOME.MonoJoin(PANICS(v.UpdatePointer(PTNIL)))
	}

	// If the points-to set excluding nil is not empty,
	// dereferencing may succeed, and the points-to set for
	// the successful outcome does not contain nil.
	if pt := pt.FilterNil(); !pt.Empty() {
		OUTCOME = OUTCOME.MonoJoin(SUCCEEDS(v.UpdatePointer(pt)))
	}

	return OUTCOME
}

// Load models dereferencing an abstract pointer value. It performs
// the least-upper bound for all the possible values mapped in memory
// to members of the abstract pointer's points-to set.
func Load(v L.AbstractValue, mem L.Memory) (res L.AbstractValue) {
	pt := v.PointerValue()

	res = L.Consts().BotValue()
	mops := L.MemOps(mem)

	pt.ForEach(func(l loc.Location) {
		res = mops.GetUnsafe(l).MonoJoin(res)
	})

	return res
}
