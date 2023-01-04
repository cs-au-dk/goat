package lattice

import loc "github.com/cs-au-dk/goat/analysis/location"

// Short-hand reprsentations of known lattice constants.
// Flat elements cannot not be statefully manipulated
// in external sources. Passing shallows copies is safe.
var (
	_CONST_MUTEX_UNLOCKED = elFact.Flat(mutexLattice)(false)
	_CONST_MUTEX_LOCKED   = elFact.Flat(mutexLattice)(true)
	_CONST_STATUS_OPEN    = elFact.Flat(channelInfoLattice.Status())(true)
	_CONST_STATUS_CLOSED  = elFact.Flat(channelInfoLattice.Status())(false)
)

// consts is a factory of common lattice element constants.
type consts struct{}

// Consts yields a factory for commonly used lattice element constants.
// WARNING: Not to be confused with constant propagation lattice.
func Consts() consts {
	return consts{}
}

// BotValue yields ⊥ for the abstract value lattice.
func (c consts) BotValue() AbstractValue {
	return valueLattice.Bot().AbstractValue()
}

// WilcardValue yields the wildcard value of the abstract value lattice.
func (c consts) WildcardValue() AbstractValue {
	return elFact.AbstractWildcard()
}

// BasicTopValue yields an abstract value embedding the ⊤ value in the constant propagation lattice.
func (c consts) BasicTopValue() AbstractValue {
	return elFact.AbstractBasic(0).ToTop()
}

// AbstractBasicBooleans yields the abstract values representing the abstract `true` and `false`
// boolean constants.
func (c consts) AbstractBasicBooleans() (TRUE, FALSE AbstractValue) {
	return Create().Element().AbstractBasic(true),
		Create().Element().AbstractBasic(false)
}

// ForChanStatus returns the abstract constants representing known channel status values, OPEN and CLOSED.
func (c consts) ForChanStatus() (OPEN, CLOSED FlatElement) {
	return c.Open(), c.Closed()
}

// Open returns the abstract constant representing the known channel status OPEN.
func (c consts) Open() FlatElement {
	return _CONST_STATUS_OPEN
}

// Closed returns the abstract constant representing the known channel status CLOSED.
func (c consts) Closed() FlatElement {
	return _CONST_STATUS_CLOSED
}

// Mutex returns the abstract constants representing known Mutex values, LOCKED and UNLOCKED.
func (c consts) Mutex() (LOCKED, UNLOCKED FlatElement) {
	return c.Locked(), c.Unlocked()
}

// PointsToNil returns the known singleton points-to set containing `nil`.
func (c consts) PointsToNil() PointsTo {
	return Create().Element().PointsTo(loc.NilLocation{})
}

// Locked returns the abstract constant representing the known Mutex status LOCKED.
func (c consts) Locked() FlatElement {
	return _CONST_MUTEX_LOCKED
}

// Unlocked returns the abstract constant representing the known Mutex status UNLOCKED.
func (c consts) Unlocked() FlatElement {
	return _CONST_MUTEX_UNLOCKED
}

// OpOutcomes returns the BLOCKS basic outcome, which can also be used as a baseline when extending
// the outcomes of abstract operations, and the outcome factories:
//
//	SUCCEEDS, PANICS : AbstractValue → OpOutcomes
//
// Where, for any abstract value v, we have that:
//  1. SUCCEEDS(v) = [ OK ↦ v ]
//  2. PANICS(v) = [ PANIC ↦ v ]
func (c consts) OpOutcomes() (
	BLOCKS OpOutcomes,
	SUCCEEDS, PANICS func(AbstractValue) OpOutcomes,
) {
	return c.OpBlocks(), c.OpSucceeds(), c.OpPanics()
}

// OpBlocks returns the BLOCKS basic outcome, which can also be used as a baseline when extending
// the outcomes of abstract operations. The BLOCKS outcome is the map of unbound outcomes,
// BLOCKS = [ OK ↦ ⊥, PANIC ↦ ⊥ ]
func (c consts) OpBlocks() OpOutcomes {
	return elFact.OpOutcomes()
}

// OpSucceeds returns the outcome factory SUCCEEDS, where, for any abstract value v,
// and outcome O = SUCCEEDS(v) ∈ OpOutcomes, we have that O = [ OK ↦ v, PANIC ↦ ⊥ ].
func (c consts) OpSucceeds() func(AbstractValue) OpOutcomes {
	return func(val AbstractValue) OpOutcomes {
		outcome := elFact.OpOutcomes()
		outcome.mp = outcome.mp.Update(_OUTCOME_OK, val)
		return outcome
	}
}

// OpPanics returns the outcome factory PANICS, where, for any abstract value v,
// and outcome O = PANICS(v) ∈ OpOutcomes, we have that O = [ OK ↦ ⊥, PANIC ↦ v ].
func (c consts) OpPanics() func(AbstractValue) OpOutcomes {
	return func(val AbstractValue) OpOutcomes {
		outcome := elFact.OpOutcomes()
		outcome.mp = outcome.mp.Update(_OUTCOME_PANIC, val)
		return outcome
	}
}
