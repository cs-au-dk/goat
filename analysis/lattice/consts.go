package lattice

import loc "Goat/analysis/location"

// Flat elements cannot not be statefully manipulated
// in external sources. Passing shallows copies is safe.
var (
	_CONST_MUTEX_UNLOCKED = elFact.Flat(mutexLattice)(false)
	_CONST_MUTEX_LOCKED   = elFact.Flat(mutexLattice)(true)
	_CONST_STATUS_OPEN    = elFact.Flat(channelInfoLattice.Status())(true)
	_CONST_STATUS_CLOSED  = elFact.Flat(channelInfoLattice.Status())(false)
)

type consts struct{}

var _consts = consts{}

// Commonly used constant factory.
// WARNING: Not to be confused with constant propagation lattice.
func Consts() consts {
	return _consts
}

func (c consts) BotValue() AbstractValue {
	return valueLattice.Bot().AbstractValue()
}

func (c consts) WildcardValue() AbstractValue {
	return elFact.AbstractWildcard()
}

func (c consts) BasicTopValue() AbstractValue {
	return elFact.AbstractBasic(0).ToTop()
}

func (c consts) AbstractBasicBooleans() (TRUE, FALSE AbstractValue) {
	return Create().Element().AbstractBasic(true),
		Create().Element().AbstractBasic(false)
}

func (c consts) ForChanStatus() (OPEN, CLOSED FlatElement) {
	return c.Open(), c.Closed()
}

func (c consts) Open() FlatElement {
	return _CONST_STATUS_OPEN
}

func (c consts) Closed() FlatElement {
	return _CONST_STATUS_CLOSED
}

func (c consts) Mutex() (LOCKED, UNLOCKED FlatElement) {
	return c.Locked(), c.Unlocked()
}

func (c consts) PointsToNil() PointsTo {
	return Create().Element().PointsTo(loc.NilLocation{})
}

func (c consts) Locked() FlatElement {
	return _CONST_MUTEX_LOCKED
}

func (c consts) Unlocked() FlatElement {
	return _CONST_MUTEX_UNLOCKED
}

func (c consts) OpOutcomes() (
	BLOCKS OpOutcomes,
	SUCCEEDS, PANICS func(AbstractValue) OpOutcomes,
) {
	return c.OpBlocks(), c.OpSucceeds(), c.OpPanics()
}

func (c consts) OpBlocks() OpOutcomes {
	return elFact.OpOutcomes()
}

func (c consts) OpSucceeds() func(AbstractValue) OpOutcomes {
	return func(val AbstractValue) OpOutcomes {
		outcome := elFact.OpOutcomes()
		outcome.mp = outcome.mp.Update(_OUTCOME_OK, val)
		return outcome
	}
}

func (c consts) OpPanics() func(AbstractValue) OpOutcomes {
	return func(val AbstractValue) OpOutcomes {
		outcome := elFact.OpOutcomes()
		outcome.mp = outcome.mp.Update(_OUTCOME_PANIC, val)
		return outcome
	}
}
