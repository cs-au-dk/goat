package ops

import (
	L "github.com/cs-au-dk/goat/analysis/lattice"
)

// CondWait models stepping into .Wait() calls.
// Any .Wait() call involves checking whether the associated mutex
// may be locked. If the mutex may be unlocked, then executing
// .Wait() might result in a fatal exception.
func CondWait(val L.AbstractValue) L.OpOutcomes {
	OUTCOME, SUCCEEDS, PANICS := L.Consts().OpOutcomes()
	LOCKED, UNLOCKED := L.Consts().Mutex()
	NORLOCKS := L.Elements().FlatInt(0)

	// Behave differently based on what kind of Locker
	// is used by the Cond construct
	switch {
	case val.IsMutex():
		mu := val.MutexValue()

		// If the lock may be locked, waiting on the Cond may succeed.
		// In the follow-up the lock is guaranteed to be unlocked.
		if mu.Geq(LOCKED) {
			OUTCOME = OUTCOME.MonoJoin(SUCCEEDS(
				val.UpdateMutex(UNLOCKED),
			))
		}
		// If the lock may be unlocked, it might result in a fatal exception.
		if mu.Geq(UNLOCKED) {
			OUTCOME = OUTCOME.MonoJoin(PANICS(
				val.UpdateMutex(UNLOCKED),
			))
		}
	case val.IsRWMutex():
		mu := val.RWMutexValue()

		// If the lock may be locked, waiting on the Cond may succeed.
		// The status is guaranteed to be unlocked after starting to Wait.
		// For a RWLock, this also means that no read locks are held
		// (the lock would not have been held otherwise).
		if mu.Status().Geq(LOCKED) {
			OUTCOME = OUTCOME.MonoJoin(SUCCEEDS(
				val.UpdateRWMutex(
					mu.UpdateStatus(UNLOCKED).UpdateRLocks(NORLOCKS)),
			))
		}
		// If the lock may be unlocked, it might result in a fatal exception.
		if mu.Status().Geq(UNLOCKED) {
			OUTCOME = OUTCOME.MonoJoin(PANICS(
				val.UpdateRWMutex(
					mu.UpdateStatus(UNLOCKED)),
			))
		}
	}

	return OUTCOME
}

// CondWake models a thread trying to wake up either from a call to .Signal() or .Broadcast().
// Checks whether the associated mutex may be unlocked, in which case waking will succeed.
func CondWake(val L.AbstractValue) L.OpOutcomes {
	LOCKED, UNLOCKED := L.Consts().Mutex()
	BLOCKS, SUCCEEDS, _ := L.Consts().OpOutcomes()
	NORLOCKS := L.Elements().FlatInt(0)

	// Behave differently based on what kind of Locker is used by the Cond
	// construct
	switch {
	case val.IsMutex():
		mu := val.MutexValue()

		// If the mutex may be unlocked, then there is a success scenario
		// where the mutex is locked again.
		if mu.Geq(UNLOCKED) {
			// TODO: If the mutex status is unknown, the operation may also block?
			return SUCCEEDS(val.UpdateMutex(LOCKED))
		}
	case val.IsRWMutex():
		rmu := val.RWMutexValue()

		// If the mutex may be write unlocked, and no read locks are registered,
		// then there is a success scenario where the mutex is locked again. In that case,
		// the rw-mutex is guaranteed to have no read-locks.
		if rmu.Status().Geq(UNLOCKED) &&
			rmu.RLocks().Geq(NORLOCKS) {
			return SUCCEEDS(val.UpdateRWMutex(
				rmu.UpdateStatus(LOCKED).UpdateRLocks(NORLOCKS),
			))
		}
	}

	// If the mutex may not have the required conditions to continue, it
	// will only block.
	return BLOCKS
}
