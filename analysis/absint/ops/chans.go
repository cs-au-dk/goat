package ops

import (
	L "github.com/cs-au-dk/goat/analysis/lattice"
)

type valueTransfer = func(L.AbstractValue) L.OpOutcomes

// Abstract closing of a channel.
func Close(val L.AbstractValue) L.OpOutcomes {
	OPEN, CLOSED := L.Consts().ForChanStatus()
	BLOCK, SUCCEED, PANIC := L.Consts().OpOutcomes()

	ch := val.ChanValue()

	// Operation outcome - initially assume none.
	OUTCOME := BLOCK

	if !ch.Status().Eq(OPEN) {
		// If channel is not guaranteed open, then the outcome includes a panic scenario
		// where the value of the channel is the least between its current value or closed.
		upd := val.Update(ch.MeetStatus(CLOSED))
		OUTCOME = OUTCOME.MonoJoin(PANIC(upd))
	}
	if ch.Status().Geq(OPEN) {
		// If the channel may be open, then the outcome includes a success scenario
		// where the channel is guaranteed closed.
		upd := val.Update(ch.UpdateStatus(CLOSED))
		OUTCOME = OUTCOME.MonoJoin(SUCCEED(upd))
	}
	return OUTCOME
}

// Abstract transformation on a single channel abstract buffer element.
func FlatSend(payload L.AbstractValue) valueTransfer {
	OPEN, CLOSED := L.Consts().ForChanStatus()
	BLOCK, SUCCEED, PANIC := L.Consts().OpOutcomes()

	return func(val L.AbstractValue) L.OpOutcomes {
		// Channel related values
		ch := val.ChanValue()

		// Initially assume the operation will block. If none
		// of the conditions are met for non-blocking behaviour, sending
		// is guaranteed to be impossible at that point.
		OUTCOME := BLOCK

		switch {
		case ch.Status().IsBot():
			// If the channel is guaranteed nil, then only blocking behaviour may
			// follow
			return BLOCK
		case ch.Status().Is(CLOSED):
			// If the channel is guaranteed closed, then only a panic scenario
			// is required. The incoming payload value is discarded.
			return PANIC(val)
		case ch.Status().IsTop():
			// If the channel may be closed, the sending operation might include a panic
			// scenario. The status is guaranteed to be closed for that outcome.
			// The incoming payload value is discarded.
			upd := val.Update(ch.UpdateStatus(CLOSED))
			OUTCOME = OUTCOME.MonoJoin(PANIC(upd))
		}
		// Proceed to the succeess scenario.
		// Sends require information about the capacity of a channel.
		cap := ch.Capacity()
		// Since only success scenarios will follow, the channel is guaranteed to be
		// open for all upcoming outcomes.
		ch = ch.UpdateStatus(OPEN)

		switch {
		case cap.IsBot() || ch.BufferFlat().IsBot():
			// Bottom capacity or flat buffer MIGHT indicate a nil channel.
			// Sending on nil channels always blocks.
			// TODO: This should technically be unreachable due to checking whether
			// status is bot first. (They should both be simultaneously bot, or neither)
			return BLOCK
		case cap.IsTop():
			// If capacity is unknown, the flat buffer should also be top. Fallthrough
			// on "buffer is top" case.
			fallthrough
		case ch.BufferFlat().IsTop():
			// If the buffer information was unknown, join the given payload
			// to the channel payload. For the outcome where sending succeeded,
			// the channel is guaranteed to be open.
			upd := val.Update(ch.JoinPayload(payload))

			return OUTCOME.MonoJoin(SUCCEED(upd))
		}

		// At this point, flat buffer information and capacity should be known values.
		// Retrieve them as numbers.
		icap := cap.FlatInt().IValue()
		buf := ch.BufferFlat().FlatInt().IValue()

		if buf < icap {
			// Increment flat buffer for channel. Since the operation is assumed to
			// succeed, the channel is also guaranteed to be open.
			ch = ch.UpdateBufferFlat(L.Create().Element().FlatInt(buf + 1))
			// If buffer is size zero, we gain precision by hard-updating the payload
			// value to the incoming value. (The incoming element is clearly the only
			// one in the buffer).
			if buf == 0 {
				ch = ch.UpdatePayload(payload)
			} else {
				// Otherwise, we must join the payload with the existing one (similarly
				// to handling array elements)
				ch = ch.JoinPayload(payload)
			}
			// Describe the updated value in the success outcome
			OUTCOME = OUTCOME.MonoJoin(SUCCEED(val.UpdateChan(ch)))
		}
		return OUTCOME
	}
}

func FlatReceive(ZERO L.AbstractValue, commaOk bool) valueTransfer {
	// If the receive is a tuple assignment, then a tuple
	// value must be propagated instead. The first member is the channel
	// with the payload value, while the second is an over-approximation
	// of whether the receive was performed on a closed channel with an empty buffer.
	recvOk := func(val L.AbstractValue, ok L.AbstractValue) L.AbstractValue {
		if commaOk {
			return L.Create().Element().AbstractStructV(val, ok)
		}
		return val
	}

	CLOSED := L.Consts().Closed()
	// Operation outcome constants (receives will not panic).
	BLOCK, SUCCEED, _ := L.Consts().OpOutcomes()
	TRUE, FALSE := L.Consts().AbstractBasicBooleans()

	return func(val L.AbstractValue) L.OpOutcomes {
		// Channel related values
		ch := val.ChanValue()
		// Channel status constants

		// No need to operate on capacity for receives.
		switch {
		case ch.BufferFlat().IsTop():
			// If the buffer information is unknown, the receive operation might not
			// block. Assume "ok" is initially "true" (no receives happened on an
			// empty buffer).
			ok := TRUE
			if ch.Status().Geq(CLOSED) {
				// If the channel may be closed, then the zero value must be
				// joined into the payload.
				ch = ch.JoinPayload(ZERO)
				// Receiving might also happen with an empty buffer, therefore ok, might
				// be either true or false.
				ok = ok.MonoJoin(FALSE)
			}

			return SUCCEED(recvOk(val.Update(ch), ok))
		case ch.BufferFlat().IsBot():
			// If the buffer information was bot, it might indicate a nil channel. The
			// outcome is a guaranteed block.
			return BLOCK
		}

		// Retrieve the flat buffer information as a known value.
		buf := ch.BufferFlat().FlatInt().IValue()

		switch {
		case buf > 0:
			// When the buffer is guaranteed not empty, decrease the buffer element. Everything else is unchanged.
			// Since the buffer is strictly positive, "ok" is guaranteed true
			ch = ch.UpdateBufferFlat(L.Create().Element().FlatInt(buf - 1))
			return SUCCEED(recvOk(val.Update(ch), TRUE))
		case ch.Status().Geq(CLOSED):
			// When buffer is empty, channel status is taken into account.
			// If the channel may be closed, there is a successful, non-blocking outcome
			// only if the channel was precisely closed (otherwise the receive would have
			// blocked, leading to no outcome). This also guarantees that "ok" is false, and the
			// payload is the zero value for the payload type.
			ch = ch.UpdateStatus(CLOSED).UpdatePayload(ZERO)
			return SUCCEED(recvOk(val.Update(ch), FALSE))
		}
		// If the buffer is empty and the channel
		// is not closed, then a receive is guaranteed to block.
		return BLOCK
	}
}

func IntervalSend(payload L.AbstractValue) valueTransfer {
	// Channel status constants
	CLOSED := L.Consts().Closed()
	OPEN := L.Consts().Open()
	// Operation outcome constants
	BLOCK := L.Consts().OpBlocks()
	SUCCEED := L.Consts().OpSucceeds()
	PANIC := L.Consts().OpPanics()
	return func(val L.AbstractValue) L.OpOutcomes {
		// Channel related values
		ch := val.ChanValue()

		// Initially assume only a blocking outcome for this channel.
		OUTCOME := BLOCK

		switch {
		case ch.Status().IsBot():
			// If the channel is nil the outcome is a guaranteed block.
			return BLOCK
		case ch.Status().Is(CLOSED):
			// If the channel is guaranteed closed, then only a panic scenario may
			// occur. The incoming payload is discarded.
			return PANIC(val)
		case ch.Status().IsTop():
			// If the channel may be closed, the sending operation might include a panic
			// scenario. The status is guaranteed to be closed for that outcome.
			// The incoming payload value is discarded.
			upd := val.Update(ch.UpdateStatus(CLOSED))
			OUTCOME = OUTCOME.MonoJoin(PANIC(upd))
		}

		// Proceed to success scenario.
		// Sends require information about the capacity of a channel.
		cap := ch.Capacity()
		// Only success from this point on, so the channel is guaranteed to be open
		// for any outcome.
		ch = ch.UpdateStatus(OPEN)

		switch {
		case cap.IsBot() || ch.BufferInterval().IsBot():
			// A bot capacity or buffer guarantees a nil channel. Sending may only
			// block.
			return BLOCK
		case cap.IsTop():
			// If buffer capacity is unknown, fallthrough on the buffer is top case.
			fallthrough
		case ch.BufferInterval().IsTop():
			// If the buffer interval is unknown, join the incoming payload to the
			// current payload. For the outcome where the sending succeeded, the
			// channel is guaranteed to be open
			upd := val.Update(ch.JoinPayload(payload))
			return OUTCOME.MonoJoin(SUCCEED(upd))
		}

		// At this point, capacity should be a known value. The buffer interval
		// should also be known, and is guaranteed to have finite bounds.
		// Axiomatically, low <= high.
		icap := cap.FlatInt().IValue()
		low, high := ch.BufferInterval().GetFiniteBounds()

		switch {
		case high < icap:
			// If the upper bound of the interval is lower than the capacity,
			// the channel is guaranteed to increment both its lower and upper bound.
			ch = ch.UpdateBufferInterval(L.Create().Element().IntervalFinite(
				low+1,
				high+1,
			))
			if high == 0 {
				// If the buffer is currently empty, the payload may be hard-updated to
				// incoming value.
				ch = ch.UpdatePayload(payload)
			} else {
				// Otherwise, the payload value must be joined to the existing one
				// (similar to array elements)
				ch = ch.JoinPayload(payload)
			}
			OUTCOME = OUTCOME.MonoJoin(SUCCEED(val.UpdateChan(ch)))
		case low < icap && high == icap:
			// If the lower bound of the interval is not yet at capacity, but the
			// upper bound is, increase only the lower bound.
			// The payload may only be joined (the channel buffer is not strictly
			// empty at this point)
			ch = ch.UpdateBufferInterval(L.Create().Element().IntervalFinite(
				low+1,
				high,
			)).JoinPayload(payload)
			OUTCOME = OUTCOME.MonoJoin(SUCCEED(val.UpdateChan(ch)))
		}

		return OUTCOME
	}
}

func IntervalReceive(ZERO L.AbstractValue, commaOk bool) valueTransfer {
	// If the receive is a tuple assignment, then a tuple
	// value must be propagated instead. The first member is the channel
	// with the payload value, while the second is an over-approximation
	// of whether the receive was performed on a closed channel with an empty buffer.
	recvOk := func(val L.AbstractValue, ok L.AbstractValue) L.AbstractValue {
		if commaOk {
			return L.Create().Element().AbstractStructV(val, ok)
		}
		return val
	}
	// Channel status constants
	CLOSED := L.Consts().Closed()
	// Operation outcome constants
	BLOCK, SUCCEED, _ := L.Consts().OpOutcomes()
	// Receive ok constants as abstract values
	TRUE := L.Create().Element().AbstractBasic(true)
	FALSE := L.Create().Element().AbstractBasic(false)

	return func(val L.AbstractValue) L.OpOutcomes {
		// Channel related values
		ch := val.ChanValue()

		// No need to operate on capacity for receives
		switch {
		case ch.BufferInterval().IsBot():
			// If the buffer interval is bot, it indicates a nil channel, which may
			// only block
			return BLOCK
		case ch.BufferInterval().IsTop():
			// If the buffer interval is unknown, we have that receives depend on the
			// status. Assume "ok" is initially "true" (no receives happen on the
			// empty buffer).
			ok := TRUE
			if ch.Status().Geq(CLOSED) {
				// If the channel may be closed, since the interval is unkown, the
				// zero value might also be the payload.
				ch = ch.JoinPayload(ZERO)
				// Since receiving might happen with an empty buffer, "ok" is also
				// unknown
				ok = ok.MonoJoin(FALSE)
			}

			// If the status is definitely open, the buffer is unknown,
			// but the payload has not yet been populated, the receive will definitely
			// block. In such a scenario, the payload will be bot, having not been
			// joined with the zero payload.
			if ch.Payload().Eq(ch.Payload().ToBot()) {
				return BLOCK
			}
			// There is only a success scenario here.
			return SUCCEED(recvOk(val.Update(ch), ok))
		}

		// If the interval buffer is a known interval,
		// retrieve it. In such a case, the interval
		// is guaranteed to have finite bounds.
		// Axiomatically, low <= high.
		low, high := ch.BufferInterval().GetFiniteBounds()

		switch {
		case low > 0:
			// If the lower bound is strictly higher than 0, decrease both.
			// The current payload is unaffected. Since the receive is guaranteed not to
			// happen on an empty channel, "ok" is guaranteed "true".
			ch = ch.UpdateBufferInterval(L.Create().Element().IntervalFinite(
				low-1,
				high-1,
			))
			return SUCCEED(recvOk(val.Update(ch), TRUE))
		case high > 0 && low == 0:
			// If the upper bound of the interval is strictly positive,
			// and the lower one is 0, decrease only the upper bound.
			ch = ch.UpdateBufferInterval(L.Create().Element().IntervalFinite(
				low,
				high-1,
			))
			// Assume "ok" is initially "true".
			ok := TRUE
			if ch.Status().Geq(CLOSED) {
				// Since the buffer might be empty at this point, if the channel might
				// be closed, then the zero value for the type may also be the payload.
				ch = ch.JoinPayload(ZERO)
				// The value of "ok" might also be false in this scenario.
				ok = ok.MonoJoin(FALSE)
			}

			return SUCCEED(recvOk(val.Update(ch), ok))
		case high == 0 && ch.Status().Geq(CLOSED):
			// If the upper bound is 0, and the channel might be closed,
			// the only successful scenario occurs when the channel is precisely
			// closed and the payload is the zero value. "ok" is also guaranteed to be
			// false in such a scenario.
			ch = ch.UpdateStatus(CLOSED).UpdatePayload(ZERO)
			return SUCCEED(recvOk(val.Update(ch), FALSE))
		}

		// In every other scenario, receiving blocks
		return BLOCK
	}
}

// Model synchronization between synchronous channels.
func Sync(commaOk bool) valueTransfer {
	// If the receive is a tuple assignment, then a tuple
	// value must be propagated instead. The first member is the channel
	// with the payload value, while the second is an over-approximation
	// of whether the receive was performed on a closed channel with an empty buffer.
	recvOk := func(val L.AbstractValue, ok L.AbstractValue) L.AbstractValue {
		if commaOk {
			return L.Create().Element().AbstractStructV(val, ok)
		}
		return val
	}

	// Channel status constants
	OPEN := L.Consts().Open()
	// Outcome related consts
	BLOCK, SUCCEED, _ := L.Consts().OpOutcomes()
	// Ok related consts
	TRUE := L.Create().Element().AbstractBasic(true)
	return func(val L.AbstractValue) L.OpOutcomes {
		// Channel related values
		ch := val.ChanValue()

		// If channel status is unknown, then the channel is nil and the operation blocks.
		if ch.Status().IsBot() {
			return BLOCK
		}

		// Do not model panics here (they should be captured by the abstract send
		// as an asynchronous channel).
		// Receives from an empty channel should be modelled by an abstract receive.

		if ch.Status().Geq(OPEN) {
			// If the channel may be open, synchronization may succeed.
			// Since we are modelling synchornization, then the capacity must be 0.
			ch = ch.UpdateCapacity(L.Create().Element().FlatInt(0))
			// The buffers should also be updated in such a case
			ch = ch.UpdateBufferFlat(L.Create().Element().FlatInt(0))
			ch = ch.UpdateBufferInterval(L.Create().Element().IntervalFinite(0, 0))
			// For the outcome where the operation succeeds, the channel is decidedly open.
			ch = ch.UpdateStatus(OPEN)

			// If the receive is a tuple assignment, the value of 'ok' will be 'true'
			return SUCCEED(recvOk(val.Update(ch), TRUE))
		}

		return BLOCK
	}
}
