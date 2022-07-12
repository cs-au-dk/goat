package absint

import (
	A "Goat/analysis/absint/ops"
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
	"Goat/utils"
	"errors"
	"fmt"
	"go/constant"
	"go/token"
	T "go/types"
	"log"
	"runtime/debug"

	"golang.org/x/tools/go/ssa"
)

func makeConstant(x interface{}) L.AbstractValue {
	return Elements().AbstractBasic(x)
}

// Make channel value from given concrete channel
// or abstract "make chan" instruction.
func makeChannelValue(
	capacity L.FlatElement,
	open bool,
	buffer int,
) L.AbstractValue {
	elem := Elements().AbstractChannel()

	var flatBuf, interval L.Element
	if capacity.IsTop() {
		flatBuf = Lattices().FlatInt().Top()
		interval = Lattices().ChannelInfo().BufferInterval().Top()
	} else {
		flatBuf = Elements().FlatInt(buffer)
		interval = Elements().IntervalFinite(
			buffer,
			buffer,
		)
	}

	return elem.Update(Elements().ChannelInfo(
		capacity,
		open,
		flatBuf,
		interval,
	))
}

// Create a channel with a bottom payload, based on the type.
// This ensures that calls to .ToTop() on the channel value will
// have the payload set to an abstract value with a determined type.
func makeChannelWithBotPayload(
	capacity L.FlatElement,
	open bool,
	buffer int,
	payloadType T.Type,
) L.AbstractValue {
	botPayload := L.ZeroValueForType(payloadType).ToBot()
	av := makeChannelValue(capacity, open, buffer)
	ch := av.ChanValue().UpdatePayload(botPayload)
	return av.Update(ch)
}

func evaluateSSA(g defs.Goro, stack L.AnalysisStateStack, val ssa.Value) L.AbstractValue {
	switch n := val.(type) {
	// The following values do not correspond to virtual registers
	case *ssa.Const:
		if cval := constant.Val(n.Value); cval != nil {
			return makeConstant(cval)
		} else {
			return Elements().AbstractPointerV(loc.NilLocation{})
		}
	case *ssa.Builtin:
		panic("Hmm.")
	case *ssa.Function:
		// Return a pointer value that contains the function.
		// The "proper" alternative would be to allocate an empty closure and return a pointer
		// to that, but that would make evaluateSSA impure.
		// ------------------
		// I actually think it's unnecessary to allocate values in the memory
		// for both closures and interfaces. Since the arguments to MakeClosure
		// and MakeInterface are already "fully" evaluated due to the SSA representation,
		// it's enough to have the thread ID, context and ssa.Value of the instruction.
		// With this information it's possible to simply look up the abstract values of the
		// arguments in the creator context, regardless of where in the program we are.
		// This would complicate abstract garbage collection a bit though, as we would no
		// longer be able to garbage collect the locals of a terminated goroutine if there
		// exists a closure/interface value that could eventually require those.
		// The upside is that we would not require a loc.FunctionPointer implementation
		// and we would have fewer allocations in the memory.
		return Elements().AbstractPointerV(loc.FunctionPointer{Fun: n})
	case *ssa.Global:
		return Elements().AbstractPointerV(loc.GlobalLocation{Site: n})
	case *ssa.FreeVar:
		// Fall through
	}

	// We can get the value from the register stored in memory
	loc := loc.LocationFromSSAValue(g, val)
	fval, found := stack.GetUnsafe(g).Get(loc)
	if found {
		return fval
	} else {
		panic("Not found " + loc.String())
	}

	/* It might be the case that an SSA register is missing in the memory for valid reasons.
	Consider this sequence of instructions:
	[ t0 = alloc chan int (ch) ]
	[ t1 = make chan int 0 ]
	[ *t0 = t1 ]
	...
	[ t2 = *t0 ]
	[ t3 = <-t2 ]
	And assume that in the concrete configuration we stopped at, the goroutine is at the
	last instruction. In this case we only have the values of local variables, parameters
	and free variables, not the values of the SSA registers. Due to the simplicity
	of the SSA conversion, we can try to evaluate the instruction backwards to figure
	out the values of t2 and t0, leading us to the local variable ch.
	*/

	/*
		// Only virtual registers should be missing!
		if _, ok := val.(ssa.Instruction); !ok {
			log.Fatalf("Non-register was missing in abstract memory: %T %v", val, val)
		}
		fval = evaluateSSA(tid, mem, val, true)
		// TODO: Ideally we would update the memory here to prevent lots of recomputations.
		// Maybe that's possible we if change the interface a bit.
		return fval
	*/

	// panic("Unreachable")
}

// TODO: Should probably just change all occurences of evaluateSSA to EvaluateSSA instead
var EvaluateSSA = evaluateSSA

func (C AnalysisCtxt) getConcPrimitivesForValue(g defs.Goro, state L.AnalysisState, ssaVal ssa.Value) (L.PointsTo, L.AnalysisState) {
	var v L.AbstractValue
	v, state = C.swapWildcard(g, state, ssaVal)
	hops := L.MemOps(state.Heap())
	ptsto := v.PointerValue()
	C.CheckPointsTo(ptsto)

	if ptsto.Empty() {
		log.Fatalln("PointsTo set is unexpectedly empty", ssaVal, ssaVal.Parent())
	}

	getPrim := func(l loc.Location) L.AbstractValue {
		val, found := hops.Get(l)
		if !found {
			log.Fatalf("Location in points-to set for concurrency primitive value is not found?\n%T %v", l, l)
		}

		return val
	}

	if utils.IsNamedType(ssaVal.Type(), "sync", "Locker") {
		res := Elements().PointsTo()

		for _, primLoc := range ptsto.NonNilEntries() {
			l, ok := primLoc.(loc.AllocationSiteLocation)
			if !ok {
				log.Fatalf("%v ??? %v %T %s", ssaVal, primLoc, primLoc, string(debug.Stack()))
			}

			if l.Site != nil {
				if v, ok := l.Site.(ssa.CallInstruction); ok &&
					v.Common().Value.Name() == "RLocker" {

					res = res.Add(l)
					continue
				}
			}

			val := getPrim(l)

			switch {
			case val.IsPointer():
				res = res.MonoJoin(val.PointerValue())
			case val.IsWildcard():
				val, state = C.swapWildcardLoc(g, state.UpdateHeap(hops.Memory()), l)
				hops = L.MemOps(state.Heap())
				res = res.MonoJoin(val.PointerValue())
			}
		}
		C.CheckPointsTo(res)

		return res, state.UpdateHeap(hops.Memory())
	}

	// FIXME: This is an additional safety check to detect bugs early.
	for _, primLoc := range ptsto.NonNilEntries() {
		val := getPrim(primLoc)

		switch {
		case val.IsChan() && val.ChanValue().Status().IsBot():
			log.Fatalf(
				"A returned pointer for a channel primitive does not point to a proper ChannelInfo object?\nGot: %v",
				val,
			)
		case val.IsMutex() && val.MutexValue().IsBot():
			log.Fatalf(
				"A returned pointer for a mutex primitive does not point to a proper Mutex object?\nGot: %v",
				val,
			)
		case val.IsRWMutex() && val.RWMutexValue().RLocks().IsBot():
			log.Fatalf(
				"A returned pointer for a RW-mutex primitive does not point to a proper RWMutex object?\nGot: %v",
				val,
			)
		}
	}

	return ptsto, state.UpdateHeap(hops.Memory())
}

var ErrFocusedPrimitiveSwapped = errors.New("Focused primitive was wildcard swapped")
var ErrUnboundedGoroutineSpawn = errors.New("Unbounded goroutine spawns detected")

func (C AnalysisCtxt) swapWildcardLoc(g defs.Goro, state L.AnalysisState, l loc.AddressableLocation) (
	L.AbstractValue, L.AnalysisState,
) {
	state = A.SwapWildcard(g, C.LoadRes.Pointer, state, l)
	C.LogWildcardSwap(state.Heap(), l)
	av := state.GetUnsafe(l)

	if C.Metrics.Enabled() && C.FocusedPrimitives != nil {
		ptsto := av.PointerValue()
		for _, prim := range C.FocusedPrimitives {
			if ptsto.Contains(loc.AllocationSiteLocation{
				Goro:    defs.Create().TopGoro(),
				Context: prim.Parent(),
				Site:    prim,
			}) && !opts.NoAbort() {
				// Panic also closes the skipped channel such that analysis aborts immediately.
				C.Metrics.Panic(fmt.Errorf("%w: %v at %v", ErrFocusedPrimitiveSwapped, prim, l))
				break
			}
		}
	}

	return av, state
}

func (C AnalysisCtxt) swapWildcard(g defs.Goro, state L.AnalysisState, v ssa.Value) (L.AbstractValue, L.AnalysisState) {
	av := evaluateSSA(g, state.Stack(), v)

	if av.IsWildcard() {
		l := loc.LocationFromSSAValue(g, v)
		av, state = C.swapWildcardLoc(g, state, l)
	}

	return av, state
}

// Used for implementing ssa.FieldAddr and ssa.IndexAddr.
// A field of -2 indicates IndexAddr.
func (C AnalysisCtxt) wrapPointers(g defs.Goro, state L.AnalysisState, ssaPtr ssa.Value, field int) (
	ret L.AbstractValue, newState L.AnalysisState, hasNil bool,
) {
	entries := []loc.Location{}
	v, state := C.swapWildcard(g, state, ssaPtr)
	base := v.PointerValue()
	C.CheckPointsTo(base)

	if base.Empty() {
		fmt.Println(v, g, state, ssaPtr, ssaPtr.Type(), ssaPtr.Parent())
		_, ok := C.LoadRes.Pointer.Queries[ssaPtr]
		fmt.Println(ok)
		log.Fatalln("Empty points-to set for wrapPointers?", ssaPtr)
	}

	base = base.FilterNilCB(func() { hasNil = true })

	for _, ptr := range base.Entries() {
		// Safety check
		if aval := L.MemOps(state.Heap()).GetUnsafe(ptr); field != -2 &&
			!(aval.IsStruct() || aval.IsCond()) {
			panic(fmt.Sprintf("Tried to make a field pointer to a non-struct value: %v\n"+
				"Bound to location: %s", aval, ptr))
		}

		// if field != -2 {
		entries = append(entries, loc.FieldLocation{Base: ptr, Index: field})
		// } else {
		// 	entries = append(entries, loc.FieldLocation{Base: ptr, Index: -2})
		// }
	}

	return Elements().AbstractPointer(entries), state, hasNil
}

// Determine what are the possible successors via a single silent transition.
// Models the effect of the ssa instruction on the abstract memory.
func (s *AbsConfiguration) singleSilent(C AnalysisCtxt, g defs.Goro, cl defs.CtrLoc, state L.AnalysisState) (
	succs L.AnalysisIntraprocess,
) {
	stack, heap := state.Stack(), state.Heap()

	//------------------------------------------------------
	//                    Helpers
	//------------------------------------------------------
	swapWildcard := func(mem L.AnalysisState, v ssa.Value) (L.AbstractValue, L.AnalysisState) {
		return C.swapWildcard(g, mem, v)
	}
	evalSSA := func(v ssa.Value) L.AbstractValue {
		return evaluateSSA(g, stack, v)
	}

	defer func() {
		// Provide some additional debugging support when things fail...
		if err := recover(); err != nil {
			var typI interface{} = cl.Node()
			if n, ok := cl.Node().(*cfg.SSANode); ok {
				typI = n.Instruction()
			}

			log.Printf("Abstract interpretation of %T %s in function %s failed.\n%s\nOperands:\n", typI, cl.Node(), cl.Node().Function(), err)

			fmt.Println(StringifyNodeArguments(g, stack, cl.Node()))

			cfg.PrintNodePosition(cl.Node(), C.LoadRes.Cfg.FileSet())

			fmt.Println(state)

			if opts.Visualize() {
				cfg.VisualizeFunction(cl.Node().Function())
			}

			panic(err)
		}
	}()

	//------------------------------------------------------
	//				Successor dependent transfer functions
	//------------------------------------------------------
	succs = Elements().AnalysisIntraprocess()

	noop := func() {
		for succ := range cl.Successors() {
			succs = succs.Update(succ, state)
		}
	}

	singleUpd := func(state L.AnalysisState) {
		// C.CheckMemory(state.Stack())
		C.CheckMemory(state.Heap())
		succs = succs.Update(cl.Successor(), state)
	}

	singleStackUp := func(stack L.AnalysisStateStack) {
		singleUpd(state.UpdateStack(stack))
	}

	// Filter the set of successors such that we only proceed to "charged" defer calls
	filterDeferSuccessors := func() (ret []defs.CtrLoc) {
		charged, _ := state.ThreadCharges().Get(g)
		for succ := range cl.Successors() {
			ok := true
			switch n := succ.Node().(type) {
			case *cfg.DeferCall:
				_, ok = charged.Get(succ)
			case *cfg.BuiltinCall:
				if n.IsDeferred() {
					_, ok = charged.Get(succ)
				}
			}

			if ok {
				ret = append(ret, succ)
			}
		}
		return
	}

	wrapPointers := func(ssaVal ssa.Value, field int) (L.AbstractValue, L.AnalysisState) {
		ret, state, hasNil := C.wrapPointers(g, state, ssaVal, field)
		if hasNil {
			succs = succs.Update(cl.Panic(), state)
		}
		return ret, state
	}

	deref := func(state L.AnalysisState, ptr ssa.Value) (L.AbstractValue, L.AnalysisState) {
		// Retrieve the abstract value of that location
		av, state := swapWildcard(state, ptr)

		res := L.Consts().BotValue()
		A.ToDeref(av).
			OnSucceed(func(av L.AbstractValue) {
				res = A.Load(av, state.Heap())
			}).
			OnPanic(func(_ L.AbstractValue) {
				succs = succs.Update(cl.Panic(), state)
			})

		return res, state
	}

	updatePhiNodes := func(fromBlock, toBlock *ssa.BasicBlock) L.AnalysisState {
		// Update phi nodes in toBlock with the values coming from fromBlock.
		predIdx := -1
		for i, pred := range toBlock.Preds {
			if pred == fromBlock {
				predIdx = i
				break
			}
		}

		if predIdx == -1 {
			panic("???")
		}

		// From the documentation:
		// Within a block, all φ-nodes must appear before all non-φ nodes.
		stack := state.Stack()
		for _, instr := range toBlock.Instrs {
			if phi, ok := instr.(*ssa.Phi); ok {
				stack = stack.UpdateLoc(
					loc.LocationFromSSAValue(g, phi),
					evalSSA(phi.Edges[predIdx]),
				)
			} else {
				break
			}
		}

		return state.UpdateStack(stack)
	}

	//-----------------------------------------------------------
	//							Control location case analysis
	//-----------------------------------------------------------

	C.LogCtrLocMemory(g, cl, state.Heap())
	switch n := cl.Node().(type) {
	case *cfg.FunctionEntry:
		noop()
	case *cfg.PostCall:
		if cl.Exiting() {
			stack := state.ThreadCharges().GetUnsafe(g).GetUnsafe(cl)
			// If the exiting flag is set we should immediately begin processing deferred calls.
			// We set the return value to bottom to avoid crashes when looking it up.
			stack = stack.Update(loc.ReturnLocation(g, n.Function()), L.Consts().BotValue())
			succs = succs.Update(
				cl.Derive(n.PanicCont()),
				state.UpdateThreadStack(g, stack),
			)
		} else {
			noop()
		}

	case *cfg.BuiltinCall:
		switch n.Builtin().Name() {
		case "recover":
			// Optimistically assume built-in recover calls are always performed
			// when not panicking, making the results nil. Any branching on the
			// recover pointer will resolve to the nil branch
			singleStackUp(stack.UpdateLoc(
				loc.LocationFromSSAValue(g, n.Call.Value()),
				L.Elements().AbstractPointerV(loc.NilLocation{}),
			))
		case "append":
			// BaseV contains a set of pointers to possible base arrays
			slice, apps := n.Args()[0], n.Args()[1]
			v, newState := swapWildcard(state, slice)
			state = newState

			baseV := v.PointerValue()

			C.CheckPointsTo(baseV)
			var argVs L.AbstractValue
			if t, ok := apps.Type().Underlying().(*T.Basic); ok &&
				t.Info()&T.IsString != 0 {
				argVs = A.TypeAdapter(apps.Type(), slice.Type(), evalSSA(apps))
			} else {
				argVs, state = deref(state, apps)
			}

			hops := L.MemOps(heap)

			if baseV.Empty() {
				log.Fatalln("Empty points-to set for append call?", n.Args()[0])
			}

			// "append" may silently allocate a new backing array for the slice if the
			// capacity is overflowed. This means that points-to sets for slices may
			// grow quite large if we reflect this behavior directly in the abstract interpreter.
			// Since our abstract model of arrays and slices is very coarse, we do not
			// gain any precision from modeling this behavior, and soundness is also preserved.
			// (As long as we do not implement precise results for slice equality)
			// As a special case we abstractly allocate a new array if the points-to
			// set only includes nil.

			rval := n.Call.Value()
			eType := rval.Type().Underlying().(*T.Slice).Elem()
			if baseV.Contains(loc.NilLocation{}) && baseV.Size() == 1 {
				allocSite := loc.AllocationSiteLocation{
					Goro:    g,
					Context: n.Function(),
					Site:    rval,
				}
				newState = newState.HeapAlloc(allocSite,
					L.Elements().AbstractArray(
						L.ZeroValueForType(eType)))
				baseV = baseV.Add(allocSite)
			}

			// We transfer the argument values to the possible base arrays
			for _, ptr := range baseV.NonNilEntries() {
				hops.WeakUpdate(ptr, argVs)
			}

			// append returns a slice (which in our case still points to the same base array)
			singleUpd(state.Update(
				loc.LocationFromSSAValue(g, rval),
				L.Consts().BotValue().Update(baseV),
			).UpdateHeap(hops.Memory()))

		case "ssa:wrapnilchk":
			// wrapnilchk returns ptr if non-nil, panics otherwise.
			// (For use in indirection wrappers.)
			argV := evalSSA(n.Args()[0])
			if argV.IsWildcard() {
				singleStackUp(stack.UpdateLoc(
					loc.LocationFromSSAValue(g, n.Call.Value()),
					argV,
				))
			} else {
				if ptrV := argV.PointerValue().FilterNilCB(func() {
					succs = succs.Update(cl.Panic(), state)
				}); !ptrV.Empty() {
					singleStackUp(stack.UpdateLoc(
						loc.LocationFromSSAValue(g, n.Call.Value()),
						argV.UpdatePointer(ptrV),
					))
				}
			}

		default:
			singleUpd(spoofCall(g, n.Call, state))
		}

	case *cfg.DeferCall:
		succs = C.callSuccs(g, cl, state)

	case *cfg.PostDeferCall:
		// For deferred calls we must filter the successors based on which defers are charged.
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, state)
		}

	case *cfg.BlockEntry:
		noop()

	case *cfg.BlockExit:
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, state)
		}

	case *cfg.BlockEntryDefer:
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, state)
		}

	case *cfg.BlockExitDefer:
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, state)
		}

	case *cfg.SSANode:
		switch insn := n.Instruction().(type) {

		case *ssa.Defer:
			newState := state
			// In programs such as this:
			// 	defer close(ch)
			// 	select {}
			// the defer operation will not have a defer link.
			if dfr := n.DeferLink(); dfr != nil {
				// Charge deferlink in both exiting and non-exiting states
				ncl := cl.Derive(dfr)
				stack := stack.GetUnsafe(g)
				newState = state.AddCharges(g,
					L.Charge{ncl.WithExiting(false), stack},
					L.Charge{ncl.WithExiting(true), stack})
			}

			succs = succs.Update(cl.Successor(), newState)

		case ssa.Value:
			// Declared here to reduce code duplication.
			allocSite := loc.AllocationSiteLocation{
				Goro:    g,
				Context: insn.Parent(),
				Site:    insn,
			}

			res := L.Consts().BotValue()

			switch val := insn.(type) {
			case *ssa.Alloc:
				eT := val.Type().Underlying().(*T.Pointer).Elem()
				initVal := L.ZeroValueForType(eT)

				// TODO: When allocating a struct we should check whether it has
				// communication primitives that should be set to ⊤ according to
				// C.IsPrimitiveFocused.

				if val.Heap {
					state = state.HeapAlloc(allocSite, initVal)
				} else {
					state = state.Update(allocSite, initVal)
				}

				res = Elements().AbstractPointerV(allocSite)
			case *ssa.MakeChan:
				C.Metrics.AddChan(cl)
				capValue := evalSSA(val.Size).BasicValue()
				if !(capValue.IsBot() || capValue.IsTop()) {
					// Convert from *flatElement to *FlatIntElement
					capValue = Elements().FlatInt(int(capValue.Value().(int64)))
				}

				plType := insn.Type().Underlying().(*T.Chan).Elem()
				ch := makeChannelWithBotPayload(capValue, true, 0, plType)

				// If the channel is not in the set of FocusedPrimitives, allocate a Top value instead.
				if !C.IsPrimitiveFocused(insn) {
					topCh := ch.ToTop().ChanValue()
					state = state.Allocate(allocSite, ch.UpdateChan(
						// Prevent the use of DroppedTop elements for
						// channels that have struct payloads.
						// We use a struct with top fields instead.
						topCh.UpdatePayload(L.TopValueForType(plType)),
					), true)
					res = Elements().AbstractPointerV(allocSite)
				} else {
					state = state.HeapAlloc(allocSite, ch)
					res = L.Elements().AbstractPointerV(allocSite)
				}
			case *ssa.UnOp:
				switch val.Op {
				case token.MUL:
					// Pointer dereference operation
					res, state = deref(state, val.X)

				case token.ARROW:
					// Receive on irrelevant channel
					res = L.TopValueForType(insn.Type())

				default:
					res = A.UnOp(evalSSA(val.X), val)
				}

			case *ssa.BinOp:
				// No wildcard swap because it is unlikely to improve precision.
				// (Which can only happen if the wildcard represents 0 allocation sites).
				x, y := evalSSA(val.X), evalSSA(val.Y)
				res = A.BinOp(state, x, y, val)

				/*
					Special case loops guards for loops over slices to avoid false positives
					in cases such as BlockingAnalysis/matching-for-loops.
					A for loop over a slice looks like this:
						t0 = len(ints) at position: -
						jump 1 at position: -
						1 :
						t1 = phi [0: -1:int, 2: t2] at position: -
						t2 = t1 + 1:int at position: -
						t3 = t2 < t0 at position: -
						if t3 goto 2 else 3 at position: -
						2 :
						t4 = &ints[t2] at position: main.go:13:20
						t5 = *t4 at position: -
						t6 = println(t2, t5) at position: main.go:14:10
						jump 1 at position: -
					Notably all the instructions related to the loop guard have invalid positions.
					We match on this specific pattern and force the guard to succeed when
					t2 = 0 (i.e. the first iteration), unless the argument to len is definitely nil.
				*/
				if call, ok := val.Y.(*ssa.Call); ok && res.BasicValue().IsTop() &&
					val.Op == token.LSS && val.Pos() == token.NoPos &&
					val.X.Pos() == token.NoPos && val.Y.Pos() == token.NoPos &&
					x.BasicValue().Is(int64(0)) && len(*val.Referrers()) == 1 {

					_, isIf := (*val.Referrers())[0].(*ssa.If)
					ptNil := Elements().AbstractPointerV(loc.NilLocation{})
					if blt, ok := call.Call.Value.(*ssa.Builtin); ok &&
						blt.Name() == "len" && isIf &&
						!evalSSA(call.Call.Args[0]).Eq(ptNil) {
						res = Elements().AbstractBasic(true)
					}
				}

			case *ssa.MakeClosure:
				// Put free variables in the struct
				bindings := make(map[interface{}]L.Element)
				for i, value := range val.Bindings {
					bindings[i] = evalSSA(value)
				}

				state = state.HeapAlloc(allocSite, Elements().AbstractClosure(val.Fn, bindings))
				res = Elements().AbstractPointerV(allocSite)
			case *ssa.MakeSlice:
				// Used to create slices of dynamic length (and capacity).
				eType := val.Type().Underlying().(*T.Slice).Elem()
				state = state.HeapAlloc(allocSite, Elements().AbstractArray(L.ZeroValueForType(eType)))
				res = L.Elements().AbstractPointerV(allocSite)

			case *ssa.MakeMap:
				typ := val.Type().Underlying().(*T.Map)
				kTyp, vTyp := typ.Key(), typ.Elem()

				state = state.HeapAlloc(allocSite,
					// The current abstract of maps only separates keys and values.
					Elements().AbstractMap(
						L.ZeroValueForType(kTyp).ToBot(),
						L.ZeroValueForType(vTyp).ToBot()))
				res = Elements().AbstractPointerV(allocSite)

			case *ssa.Lookup:
				// Lookup can be used on both strings and maps.
				if bt, ok := val.X.Type().Underlying().(*T.Basic); ok && bt.Info()&T.IsString != 0 {
					if val.CommaOk {
						panic("What?")
					}

					// If the subject of a lookup is a string, we simply return top.
					res = L.TopValueForType(val.Type())
				} else {
					// Otherwise the subject is a map, so we perform a lookup in all the
					// pointed-to maps.
					base, newState := swapWildcard(state, val.X)
					state = newState
					maps := base.PointerValue()

					if maps.Empty() {
						log.Fatalln("Empty points-to set for lookup?")
					}

					elemType := val.Type()
					if val.CommaOk {
						elemType = elemType.(*T.Tuple).At(0).Type()
					}

					// We might return the zero-value if the key is not found
					// or we are looking up in a nil map.
					res = L.ZeroValueForType(elemType)

					// Opportunity for refinement:
					// If evalSSA(key) ⊓ mapV.keys = ⊥ , the lookup will definitely miss.

					for _, ptr := range maps.NonNilEntries() {
						mapV := state.GetUnsafe(ptr).StructValue()
						res = res.MonoJoin(mapV.Get("values").AbstractValue())
					}

					if val.CommaOk {
						res = Elements().AbstractStructV(
							res,
							makeConstant(true).MonoJoin(makeConstant(false)),
						)
					}
				}

			case *ssa.Next:
				if val.IsString {
					TOP := makeConstant(false).ToTop()
					iter := evalSSA(val.Iter).BasicValue()

					switch {
					case iter.IsTop():
						res = Elements().AbstractStructV(TOP, TOP, TOP)
					case iter.IsBot():
						panic("What?")
					default:
						str, ok := iter.Value().(string)
						if !ok {
							panic("What?")
						}

						if len(str) == 0 {
							res = Elements().AbstractStructV(
								makeConstant(false),
								makeConstant(false).ToBot(),
								makeConstant(false).ToBot())
						} else {
							res = Elements().AbstractStructV(TOP, TOP, TOP)
						}
					}
				} else {
					v := evalSSA(val.Iter)
					tupleT := val.Type().(*T.Tuple)

					if v.IsWildcard() {
						// fmt.Println(val.Type())
						res = L.TopValueForType(val.Type())
						break
					}
					res = Elements().AbstractStructV(
						makeConstant(false),
						L.Consts().BotValue(),
						L.Consts().BotValue())

					v, newState := swapWildcard(state, val.Iter)
					state = newState

					bases := v.PointerValue()
					for _, ptr := range bases.NonNilEntries() {
						mapV := state.GetUnsafe(ptr).StructValue()
						keyV := mapV.Get("keys").AbstractValue()

						if keyV.IsBot() || keyV.Eq(keyV.ToBot()) {
							// Skip if the map is guaranteed to be empty
							continue
						}

						valV := mapV.Get("values").AbstractValue()
						// If keys or values are excluded from the iteration,
						// they get type T.Invalid.
						// We skip joining in those cases.
						if T.Identical(tupleT.At(1).Type(), T.Typ[T.Invalid]) {
							keyV = L.Consts().BotValue()
						}
						if T.Identical(tupleT.At(2).Type(), T.Typ[T.Invalid]) {
							valV = L.Consts().BotValue()
						}

						res = res.MonoJoin(
							Elements().AbstractStructV(makeConstant(true), keyV, valV),
						)
					}
				}

			case *ssa.Extract:
				tVal := evalSSA(val.Tuple)
				switch strukt := tVal.Struct().(type) {
				case *L.DroppedTop:
					typ := val.Tuple.Type().(*T.Tuple)
					TOP := L.TopValueForType(typ.At(val.Index).Type())
					res = TOP
				case L.InfiniteMap:
					res = strukt.Get(val.Index).AbstractValue()
				}

			case *ssa.ChangeType:
				res = evalSSA(val.X)

			case *ssa.ChangeInterface:
				res = evalSSA(val.X)

			case *ssa.Convert:
				inner := evalSSA(val.X)
				toT := val.Type().Underlying()

				res = func() L.AbstractValue {
					switch fromT := val.X.Type().Underlying().(type) {
					case *T.Pointer:
						// We handle conversion from pointer to unsafe.Pointer as the identity function.
						if bt, ok := toT.(*T.Basic); ok && bt.Kind() == T.UnsafePointer {
							// TODO: Since we can also get an UnsafePointer from an uintptr we should
							// probably return a flat element instead of a pointer element to avoid
							// joining incompatible abstract values. (The relation to source pointers
							// is lost anyway when going from UnsafePointer back to pointer.)
							return inner
						}
					case *T.Slice:
						switch elemType := fromT.Elem().(type) {
						case *T.Basic:
							switch elemType.Kind() {
							case T.Byte:
								fallthrough
							case T.Rune:
								// If the type converted from is a slice of bytes or runes, the
								// destination type can be string.
								switch toT := toT.(type) {
								case *T.Basic:
									if toT.Info()&T.IsString != 0 {
										return L.Consts().BasicTopValue()
									}
								}
							}
						}

					case *T.Basic:
						switch {
						case fromT.Kind() == T.UnsafePointer:
							// UnsafePointer can only convert to a pointer type
							// and uintptr.
							return L.TopValueForType(toT)
						case fromT.Info()&T.IsString != 0:
							if toT, ok := toT.(*T.Slice); ok {
								if toTe, ok := toT.Elem().(*T.Basic); ok &&
									toTe.Kind() == T.Byte || toTe.Kind() == T.Rune {
									inner = Elements().AbstractArray(L.Consts().BasicTopValue())
								}
								// We return a slice that contains the string as base...
								// This is maybe not so good.
								state = state.HeapAlloc(allocSite, inner)
								return state.GetUnsafe(allocSite)
							}

						default:
							if bas, ok := toT.(*T.Basic); ok &&
								fromT.Info()&T.IsConstType != 0 &&
								(bas.Info()&T.IsConstType != 0 || bas.Kind() == T.UnsafePointer) {
								return L.TopValueForType(bas)
							}
						}
					}

					panic(fmt.Errorf("unhandled ssa.Convert: %s", val))
				}()

			case *ssa.MakeInterface:
				state = state.HeapAlloc(allocSite, evalSSA(val.X))
				res = Elements().AbstractPointerV(allocSite)

			case *ssa.TypeAssert:
				v, newState := swapWildcard(state, val.X)
				state = newState

				okV := L.Consts().BotValue()
				zeroVal := L.ZeroValueForType(val.AssertedType)

				fbases := v.PointerValue().FilterNilCB(func() {
					// Obs: Join not needed since we're overriding L.Consts().BotValue
					res = zeroVal
					okV = makeConstant(false)
				})

				isInterfaceAssert := T.IsInterface(val.AssertedType)

				for _, ptr := range fbases.Entries() {
					site, hasSite := ptr.GetSite()
					if !hasSite {
						log.Fatalln("Pointer in TypeAssert has no site?", ptr)
					}

					success := false
					if !isInterfaceAssert {
						/* If AssertedType is a concrete type, TypeAssert
						* checks whether the dynamic type in interface X is
						* equal to it, and if so, the result of the conversion
						* is a copy of the value in the interface. */

						makeItf, ok := site.(*ssa.MakeInterface)
						if !ok {
							log.Fatalln("Allocation site did not come from a MakeInterface instruction?")
						}

						// Should we maybe use IdenticalIgnoreTags instead?
						success = T.Identical(val.AssertedType, makeItf.X.Type())
						if success {
							res = res.MonoJoin(state.GetUnsafe(ptr))
						}
					} else {
						/* If AssertedType is an interface, TypeAssert checks
						* whether the dynamic type of the interface is
						* assignable to it, and if so, the result of the
						* conversion is a copy of the interface value X. If
						* AssertedType is a superinterface of X.Type(), the
						* operation will fail iff the operand is nil. (Contrast
						* with ChangeInterface, which performs no nil-check.)
						* */

						success = T.AssignableTo(site.Type(), val.AssertedType)
						if success {
							res = res.MonoJoin(Elements().AbstractPointerV(ptr))
						}
					}

					okV = okV.MonoJoin(makeConstant(success))
					if !success {
						res = res.MonoJoin(zeroVal)
					}
				}

				// If the type assertion contains an "ok" component,
				// then the result is a pair between the value of and the result
				if val.CommaOk {
					// A tuple containing the result value and the ok flag
					res = Elements().AbstractStructV(res, okV)
				} else if makeConstant(false).Leq(okV) {
					// If the type assert may panic, add its panic successor.
					succs = succs.Update(cl.Panic(), state)
				}

			case *ssa.Field:
				res = evalSSA(val.X).StructValue().Get(val.Field).AbstractValue()

			case *ssa.FieldAddr:
				res, state = wrapPointers(val.X, val.Field)

				if res.PointerValue().Empty() && succs.Size() > 0 {
					// The points-to set only included nil
					res = L.Consts().BotValue()
				}

			case *ssa.IndexAddr:
				// TODO: Model out-of-bounds panic
				res, state = wrapPointers(val.X, -2)

				if res.PointerValue().Empty() && succs.Size() > 0 {
					// The points-to set only included nil
					// The precise result is to only include the possibility of panicking here.
					// However, we found that in the standard library slices are sometimes
					// manipulated by casting the slice to an *unsafeheader.Slice and setting
					// fields through this pointer.
					// The up-front pointer analysis does not understand this and will
					// (unsoundly) report that the slice can only be a nil-slice.
					// To prevent this unsoundness from stopping the main analysis, we force
					// the analysis to follow a path where the IndexAddr operation succeeds.
					// This requires us to come up with a "fake" location for the slice and
					// putting something there, such that following dereferences will also work.

					allocSite := loc.AllocationSiteLocation{
						Goro:    g,
						Context: val.X.Parent(),
						Site:    val.X,
					}
					// Allocate an abstract array at `allocSite` and put a top value there.
					elemType := val.Type().Underlying().(*T.Pointer).Elem()
					state = state.HeapAlloc(allocSite, Elements().AbstractArray(
						L.TopValueForType(elemType),
					))

					// Return a wrapped pointer to the element.
					res = Elements().AbstractPointerV(loc.FieldLocation{Base: allocSite, Index: -2})
				}

			case *ssa.Index:
				// TODO: Model out-of-bounds panic
				res = evalSSA(val.X).StructValue().Get(-2).AbstractValue()

			case *ssa.Slice:
				res = evalSSA(val.X)

			case *ssa.SliceToArrayPointer:
				res = evalSSA(val.X)

			case *ssa.Range:
				res = evalSSA(val.X)

			case *ssa.Phi:
				// Registers of phi nodes are updated in `updatePhiNodes`.
				// This should really be a no-op, but the code below expects res
				// to have a non-bottom value, so we copy it out and re-insert it.
				res = evalSSA(val)

			default:
				log.Fatalf("Don't know how to handle %T %v", val, val)
			}

			if !res.IsBot() {
				singleUpd(state.Update(loc.LocationFromSSAValue(g, insn), res))
			}

		case *ssa.Store:
			v, newState := swapWildcard(state, insn.Addr)
			state = newState
			addr := v.PointerValue()
			if addr.Empty() {
				panic("ssa.Store: points-to set for addr was empty")
			}

			// TODO: Maybe refactor this out into absint/ops like ToDeref/Load.
			addr = addr.FilterNilCB(func() {
				succs = succs.Update(cl.Panic(), state)
			})

			if !addr.Empty() {
				strongUpdate := L.MemOps(state.Heap()).CanStrongUpdate(addr)

				// TODO: Is it important to do the wildcard swapping here?
				val, newState := swapWildcard(state, insn.Val)
				state = newState
				hops := L.MemOps(state.Heap())
				for _, ptr := range addr.Entries() {
					if site, ok := ptr.GetSite(); ok {
						t1 := site.Type()
						t2 := insn.Addr.Type().Underlying().(*T.Pointer).Elem()
						hops.UpdateW(ptr, A.TypeAdapter(t1, t2, val), !strongUpdate)
					} else {
						hops.UpdateW(ptr, val, !strongUpdate)
					}
				}

				singleUpd(state.UpdateHeap(hops.Memory()))
			}

		case *ssa.MapUpdate:
			var v L.AbstractValue
			v, state = swapWildcard(state, insn.Map)
			maps := v.PointerValue()
			keyV, valV := evalSSA(insn.Key), evalSSA(insn.Value)

			maps = maps.FilterNilCB(func() {
				succs = succs.Update(cl.Panic(), state)
			})

			if !maps.Empty() {
				hops := L.MemOps(heap)

				for _, ptr := range maps.Entries() {
					// Merge the key and value abstract values into the map.
					hops.WeakUpdate(ptr, Elements().AbstractMap(keyV, valV))
				}

				singleUpd(state.UpdateHeap(hops.Memory()))
			}

		case *ssa.Return:
			// Put returned value in special location
			var rval L.AbstractValue

			if len(insn.Results) == 1 {
				rval = evalSSA(insn.Results[0])
			} else {
				// Make tuple
				bindings := make(map[interface{}]L.Element)
				for i, res := range insn.Results {
					bindings[i] = evalSSA(res)
				}

				rval = Elements().AbstractStruct(bindings)
			}

			rloc := loc.ReturnLocation(g, insn.Parent())

			// Return can have multiple successors if defers are under conditionals
			for _, succ := range filterDeferSuccessors() {
				succs = succs.Update(succ, state.Update(rloc, rval))
			}

		case *ssa.Go:
			// Goroutine spawning is handled by caller.
			// TODO: Perhaps we need to do top-injection here too?
			noop()

			// Spawning a goroutine can panic if the spawned function is an
			// interface method and the receiver is nil.
			succs = succs.Update(cl.Panic(), state)
		case *ssa.Jump:
			bl := insn.Block()
			singleUpd(updatePhiNodes(bl, bl.Succs[0]))
		case *ssa.Panic:
			// TODO: The value should maybe be passed on somehow?
			succs = succs.Update(cl.Panic(), state)

		case *ssa.Send:
			// Send on an irrelevant channel
			noop()

		default:
			panic(fmt.Sprintf("Don't know how to handle %T %v", insn, insn))
		}

	default:
		log.Fatalf("Don't know how to handle %T %v", n, n)
	}

	// TODO: Panic handling is broken. See the orphan-panic-bad-fix branch...
	if _, isExit := cl.Node().(*cfg.FunctionExit); succs.Size() == 0 && !isExit && !cl.Panicked() {
		cfg.PrintNodePosition(cl.Node(), C.LoadRes.Cfg.FileSet())
		if n, ok := cl.Node().(*cfg.SSANode); ok {
			log.Printf("SSA Instruction: %s %T", n.Instruction(), n.Instruction())
			if v, ok := n.Instruction().(ssa.Value); ok {
				log.Printf("Is a value: %s of type %s", v, v.Type())

				fmt.Println()
				for i, op := range n.Instruction().Operands([]*ssa.Value{}) {
					v = *op
					log.Printf("Operand %d is %s of type %s", i, v, v.Type())
					av := evaluateSSA(g, state.Stack(), v)
					log.Println("Has abstract value", av)

					if av.IsPointer() {
						for _, p := range av.PointerValue().NonNilEntries() {
							if fp, ok := p.(loc.FunctionPointer); ok {
								fmt.Println("Function pointer", fp)
							} else {
								v, _ := state.Get(p)
								fmt.Println("Location", p, "points to", v)
							}
						}
					}

					if q, ok := C.LoadRes.Pointer.Queries[v]; ok {
						log.Println("Has the following Andersen points-to set:")
						for _, l := range q.PointsTo().Labels() {
							fmt.Printf("%v\n", l.Value())
						}
						fmt.Println()
					}

					if iq, ok := C.LoadRes.Pointer.IndirectQueries[v]; ok {
						log.Println("Has the following indirect Andersen points-to set:")

						for _, l := range iq.PointsTo().Labels() {
							fmt.Printf("%v\n", l.Value())
						}
					}
				}
			}
		}
		log.Printf("Forgot to add successor? %T %v\n", cl.Node(), cl.Node())
		if opts.Visualize() {
			cfg.VisualizeFunction(cl.Node().Function())
		}
		panic("")
	}

	return
}
