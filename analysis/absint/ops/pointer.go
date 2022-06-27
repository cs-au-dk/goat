package ops

import (
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
)

// Get all the locations that could be dereferenced
func ToDeref(v L.AbstractValue) L.OpOutcomes {
	OUTCOME, SUCCEEDS, PANICS := L.Consts().OpOutcomes()
	// The points-to set including only the nil location
	PTNIL := L.Consts().PointsToNil()

	pt := v.PointerValue()

	// If the points-to set has nil, dereferencing may panic
	if pt.HasNil() {
		OUTCOME = OUTCOME.MonoJoin(PANICS(v.UpdatePointer(PTNIL)))
	}

	// If the points-to set excluding nil is not empty,
	// dereferencing may succeed
	if pt := pt.FilterNil(); !pt.Empty() {
		OUTCOME = OUTCOME.MonoJoin(SUCCEEDS(v.UpdatePointer(pt)))
	}

	return OUTCOME
}

func Load(v L.AbstractValue, mem L.Memory) (res L.AbstractValue) {
	pt := v.PointerValue()

	res = L.Consts().BotValue()
	mops := L.MemOps(mem)

	pt.ForEach(func(l loc.Location) {
		res = mops.GetUnsafe(l).MonoJoin(res)
	})

	return res
}

// Abtract store operation
// func Store(v L.AbstractValue)
