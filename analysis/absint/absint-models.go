package absint

import (
	A "Goat/analysis/absint/ops"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
	"Goat/utils"
	T "go/types"
	"log"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// Standard method invocation.
// FIXME: Relies on idiomatic Go patterns and state assumptions.
func (C AnalysisCtxt) stdInvoke(g defs.Goro,
	call ssa.CallInstruction,
	state L.AnalysisState) (res L.AnalysisState, hasModel bool) {
	method := call.Common().Method
	t := method.Type()
	switch method.Name() {
	case "Error":
		switch t := t.(type) {
		// If the method is Error() : String
		case *T.Signature:
			if t.Results().Len() == 0 {
				break
			}
			returnT, ok := t.Results().At(0).Type().Underlying().(*T.Basic)
			if !ok || returnT.Info()&T.IsString == 0 {
				break
			}

			cv := call.Value()
			if cv == nil {
				return state, true
			}
			return state.Update(loc.LocationFromSSAValue(g, cv), L.Consts().BasicTopValue()), true
		}
	case "String":
		// TODO: Code duplication here.
		switch t := t.(type) {
		// If the method is String() : String
		case *T.Signature:
			if t.Results().Len() == 0 {
				break
			}
			returnT, ok := t.Results().At(0).Type().Underlying().(*T.Basic)
			if !ok || returnT.Info()&T.IsString == 0 {
				break
			}

			cv := call.Value()
			if cv == nil {
				return state, true
			}
			return state.Update(loc.LocationFromSSAValue(g, cv), L.Consts().BasicTopValue()), true
		}
	}

	return state, false
}

// TODO: too ad-hoc...
func (C AnalysisCtxt) stdCall(
	g defs.Goro, cl defs.CtrLoc,
	call ssa.CallInstruction,
	state L.AnalysisState, fun *ssa.Function,
) (rsuccs L.AnalysisIntraprocess, hasModel bool) {
	var callLoc loc.LocalLocation
	var allocSite loc.AllocationSiteLocation
	if fun.Pkg == nil {
		return rsuccs, false
	}

	if cv := call.Value(); cv != nil {
		callLoc = loc.LocationFromSSAValue(g, cv)

		allocSite = loc.AllocationSiteLocation{
			Goro:    g,
			Context: call.Parent(),
			Site:    cv,
		}
	}

	stack, heap := state.Stack(), state.Heap()

	updMem := func(state L.AnalysisState) (L.AnalysisIntraprocess, bool) {
		return Elements().AnalysisIntraprocess().Update(cl.CallRelationNode(), state), true
	}

	evalSSA := func(v ssa.Value) L.AbstractValue {
		return evaluateSSA(g, stack, v)
	}

	// Used by time.NewTimer and time.NewTicker
	constructTimer := func() (L.AnalysisIntraprocess, bool) {
		namedTimerType := call.Value().Type().(*T.Pointer).Elem().(*T.Named)
		timerType := namedTimerType.Underlying().(*T.Struct)
		timerVal := L.ZeroValueForType(timerType)

		// Use different channel abstract values based on whether we are creating a Timer or a Ticker.
		var chVal L.AbstractValue
		if namedTimerType.Obj().Name() == "Ticker" {
			// Closing the channel is not sound, but it is an easy way to allow infinite messages.
			chVal = makeChannelValue(
				Elements().Constant(0),
				false,
				0,
			)
		} else {
			chVal = makeChannelValue(
				Elements().Constant(1),
				true,
				1,
			)
		}

		for i := 0; i < timerType.NumFields(); i++ {
			// Find the field named C (which contains the timer channel)
			if field := timerType.Field(i); field.Name() == "C" {
				payloadType := field.Type().(*T.Chan).Elem()
				// Put a zero-payload into the channel
				chVal = chVal.Update(chVal.ChanValue().UpdatePayload(
					L.ZeroValueForType(payloadType),
				))

				// Put a pointer to the channel into the struct
				mkChan, found := utils.FindSSAInstruction(fun, func(insn ssa.Instruction) bool {
					_, ok := insn.(*ssa.MakeChan)
					return ok
				})
				if !found {
					log.Fatalln("???")
				}

				hops := L.MemOps(state.Heap())

				chPtr := hops.HeapAlloc(loc.AllocationSiteLocation{
					Goro:    g,
					Context: fun,
					Site:    mkChan.(*ssa.MakeChan),
				}, chVal)
				timerVal = timerVal.Update(timerVal.StructValue().Update(i, chPtr))
				heap = hops.Memory()
				break
			}
		}

		hops := L.MemOps(heap)
		ptr := hops.HeapAlloc(allocSite, timerVal)
		newState := state.UpdateHeap(hops.Memory().Update(callLoc, ptr))
		return updMem(newState)
	}

	constructCond := func() (L.AnalysisIntraprocess, bool) {
		if len(call.Common().Args) != 1 {
			panic("what?")
		}

		lockLoc := loc.LocationFromSSAValue(g, call.Common().Args[0])
		val := Elements().AbstractCond()
		cond := val.CondValue()

		val = val.Update(cond.UpdateLocker(stack.GetLocUnsafe(lockLoc).PointerValue()))

		hops := L.MemOps(heap)
		ptr := hops.HeapAlloc(allocSite, val)

		return updMem(state.
			UpdateStack(stack.UpdateLoc(callLoc, ptr)).
			UpdateHeap(hops.Memory()))
	}

	funName := fun.String()
	switch funName {
	case "time.After":
		val := makeChannelValue(
			Elements().Constant(1),
			true,
			1,
		)
		ch := val.ChanValue()
		payloadType := call.Value().Type().(*T.Chan).Elem()
		payload := L.ZeroValueForType(payloadType)
		val = val.Update(ch.UpdatePayload(payload))

		hops := L.MemOps(heap)
		ptr := hops.HeapAlloc(allocSite, val)
		return updMem(state.
			UpdateStack(stack.UpdateLoc(callLoc, ptr)).
			UpdateHeap(hops.Memory()))

	case "time.NewTimer":
		return constructTimer()
	case "time.NewTicker":
		return constructTimer()
	case "(*sync.RWMutex).RLocker":
		if !utils.Opts().SkipSync() {
			lockVal := evalSSA(call.Common().Args[0])
			hops := L.MemOps(heap)
			ptr := hops.HeapAlloc(allocSite, lockVal)
			return updMem(state.UpdateHeap(hops.Memory().Update(callLoc, ptr)))
		}
	case "os/signal.Notify":
		// Set channel passed to Notify as a top channel.
		// TODO: This can be improved, e. g. closing a Notify channel is unsafe, since that
		// channel could be erroneously closed.
		if ch := evalSSA(call.Common().Args[0]); ch.IsWildcard() {
			return updMem(state)
		} else {
			heap = heap.LocsToTop(ch.PointerValue().NonNilEntries()...)
			return updMem(state.UpdateHeap(heap))
		}

	case "sync.NewCond":
		if !utils.Opts().SkipSync() {
			return constructCond()
		}

	case "(*sync/atomic.Value).Store":
		// atomic.Values have a field for the internal interface value that we
		// can (ab)use by storing directly into it.

		// TODO: Panic when the receiver can be nil (or maybe it is already handled earlier?)

		wrapped, state, _ := C.wrapPointers(g, state, call.Common().Args[0], 0)
		fieldPointers := wrapped.PointerValue()

		toStore, state := C.swapWildcard(g, state, call.Common().Args[1])
		// TODO: Panic when storing nil
		toStore = toStore.UpdatePointer(toStore.PointerValue().FilterNil())

		hops := L.MemOps(state.Heap())
		isWeak := !hops.CanStrongUpdate(fieldPointers)
		for _, fptr := range fieldPointers.Entries() {
			hops.UpdateW(fptr, toStore, isWeak)
		}

		return updMem(state.UpdateHeap(hops.Memory()))
	case "(*sync/atomic.Value).Load":
		// TODO: Panic when the receiver can be nil (or maybe it is already handled earlier?)
		wrapped, state, _ := C.wrapPointers(g, state, call.Common().Args[0], 0)
		return updMem(state.Update(callLoc, A.Load(wrapped, state.Heap())))

	case "runtime.Goexit",
		// Handle methods on testing.T that end the test immediately like Goexit.
		// NOTE (unsound): we ignore the calls to .String() that may happen for
		//  various arguments to these methods.
		"(*testing.common).FailNow",
		"(*testing.common).Fatal",
		"(*testing.common).Fatalf",
		"(*testing.common).SkipNow",
		"(*testing.common).Skip",
		"(*testing.common).Skipf":
		return Elements().AnalysisIntraprocess().Update(
			// Set the exiting flag to true
			cl.CallRelationNode().WithExiting(true), state,
		), true
	}

	// Implement models by a by-need basis but let us know if we need one.
	if strings.HasPrefix(funName, "(*sync/atomic.Value).") {
		log.Fatalf("Missing model for %s", funName)
	}

	return rsuccs, false
}

func spoofCall(g defs.Goro, call ssa.CallInstruction, state L.AnalysisState) L.AnalysisState {
	opts.OnVerbose(func() {
		log.Println("Spoofing call:", call, "in", call.Parent())
	})

	if val := call.Value(); val != nil {
		callLoc := loc.LocationFromSSAValue(g, val)
		return state.Update(
			callLoc,
			L.TopValueForType(val.Type()),
		)
	}

	return state
}
