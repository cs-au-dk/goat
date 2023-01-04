package absint

import (
	"errors"
	"fmt"
	"go/constant"
	"go/token"
	T "go/types"
	"log"
	"runtime/debug"

	A "github.com/cs-au-dk/goat/analysis/absint/ops"
	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

var (
	// ErrFocusedPrimitiveSwapped is emitted when a focused primitive was wildcard-swapped.
	ErrFocusedPrimitiveSwapped = errors.New("Focused primitive was wildcard swapped")
	// ErrUnboundedGoroutineSpawn is emitted when an unbounded number of goroutines is swapped
	// at the same abstract goroutine location.
	ErrUnboundedGoroutineSpawn = errors.New("Unbounded goroutine spawns detected")
)

// makeConstant creates an abstract basic value from a given constant value.
func makeConstant(x any) L.AbstractValue {
	return Elements().AbstractBasic(x)
}

// makeChannelValue creates a channel value, given the capacity, status, and buffer size.
func makeChannelValue(
	capacity L.FlatElement,
	open bool,
	buffer int,
) L.AbstractValue {
	elem := Elements().AbstractChannel()

	// If the capacity is ⊤, then the buffer size must implicitly be ⊤.
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

// makeChannelWithBotPayload creates a channel with a ⊥ payload for
// the given payload type. This ensures that calls to .ToTop() on
// the channel value will yield a ⊤ payload of the correct type.
func makeChannelWithBotPayload(
	capacity L.FlatElement,
	open bool,
	buffer int,
	payloadType T.Type,
) L.AbstractValue {
	// Get the ⊥ payload value
	botPayload := L.ZeroValueForType(payloadType).ToBot()
	av := makeChannelValue(capacity, open, buffer)

	// Update abstract channel value with ⊥ payload.
	ch := av.ChanValue().UpdatePayload(botPayload)
	return av.Update(ch)
}

// EvaluateSSA retrieves the abstract value of a given SSA value for
// a given abstract thread, given the abstract memory.
func EvaluateSSA(g defs.Goro, mem L.Memory, val ssa.Value) L.AbstractValue {
	switch n := val.(type) {
	// The following values do not correspond to virtual registers
	case *ssa.Const:
		if cval := constant.Val(n.Value); cval != nil {
			// If the constant is a primitive, construct an abstract constant
			// with the underlying value.
			return makeConstant(cval)
		} else {
			// If the constant is a pointer-like, construct an abstract points-to set
			// containing only the nil-pointer.
			return Elements().AbstractPointerV(loc.NilLocation{})
		}
	case *ssa.Builtin:
		panic("Unexpected evaluation of built-in as SSA value.")
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
		// Global variables are retrieved via indirection. Return
		// an abstract points-to set that only includes it.
		return Elements().AbstractPointerV(loc.GlobalLocation{Site: n})
	case *ssa.FreeVar:
		// Fall through
	}

	// Retrieve the value of the register stored in memory
	loc := loc.LocationFromSSAValue(g, val)
	fval, found := mem.Get(loc)
	if found {
		return fval
	}

	panic("Value of heap location " + loc.String() + " not found.")
}

// getConcPrimitivesForValue retrieves a points-to set of concurrent primitives,
// and the updated memory, given a wildcard swap.
func (C AnalysisCtxt) getConcPrimitivesForValue(sl defs.Superloc, g defs.Goro, mem L.Memory, ssaVal ssa.Value) (L.PointsTo, L.Memory) {
	// Retrieve wildcard swapped value and updated memory.s
	v, newMem := C.swapWildcard(sl, g, mem, ssaVal)
	mops := L.MemOps(newMem)

	ptsto := v.PointerValue()
	C.CheckPointsTo(ptsto)

	if ptsto.Empty() {
		log.Fatalln("PointsTo set is unexpectedly empty", ssaVal, ssaVal.Parent())
	}

	getPrim := func(l loc.Location) L.AbstractValue {
		val, found := mops.Get(l)
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
				val, newMem := C.swapWildcardLoc(sl, mops.Memory(), l)
				mops = L.MemOps(newMem)
				res = res.MonoJoin(val.PointerValue())
			}
		}
		C.CheckPointsTo(res)

		return res, mops.Memory()
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

	return ptsto, mops.Memory()
}
func (C AnalysisCtxt) swapWildcardLoc(sl defs.Superloc, mem L.Memory, l loc.Location) (
	L.AbstractValue, L.Memory,
) {
	var valToPtsTo map[ssa.Value]L.PointsTo
	if C.FocusedPrimitives != nil && opts.OptimisticFocusedSwap() {
		// Since the fragment is constructed such that it encompasses all
		// allocations of the focused primitives, we can assume that if a
		// wildcard ptsto set contains a focused primitive according to the
		// pointer analysis it must be one that was allocated during the
		// analysis.
		// Here we construct a map from focused primitive allocation sites to
		// locations that correspond to that primitive that we have seen.
		// These are then used in the wildcard swapping process to prevent
		// materialization of ⊤ versions of focused primitives.
		valToPtsTo = map[ssa.Value]L.PointsTo{}
		for _, prim := range C.FocusedPrimitives {
			ptsto := []loc.Location{}
			// Check whether each spawned goroutine has allocated the primitive.
			sl.ForEach(func(g defs.Goro, _ defs.CtrLoc) {
				aloc := loc.AllocationSiteLocation{
					Goro:    g,
					Site:    prim,
					Context: prim.Parent(),
				}
				if _, found := mem.Get(aloc); found {
					ptsto = append(ptsto, aloc)
				}
			})

			valToPtsTo[prim] = Elements().PointsTo(ptsto...)
		}
	}

	av, mem := A.UnwrapWildcard(C.LoadRes.Pointer, mem, l, valToPtsTo)
	mem = L.MemOps(mem).Update(l, av).Memory()

	C.LogWildcardSwap(mem, l)

	if C.Metrics.Enabled() && C.FocusedPrimitives != nil && !opts.OptimisticFocusedSwap() && !opts.NoAbort() {
		// Abort if the swapped in points-to set contains a focused primitive.
		av.PointerValue().ForEach(func(l loc.Location) {
			if _, isNil := l.(loc.NilLocation); isNil {
				return
			}

			if C.IsLocationFocused(l, mem) {
				// Panic also closes the skipped channel such that analysis aborts immediately.
				C.Metrics.Panic(fmt.Errorf("%w: %v", ErrFocusedPrimitiveSwapped, l))
			}
		})
	}

	return av, mem
}

func (C AnalysisCtxt) swapWildcard(sl defs.Superloc, g defs.Goro, mem L.Memory, v ssa.Value) (L.AbstractValue, L.Memory) {
	av := EvaluateSSA(g, mem, v)

	if av.IsWildcard() {
		l := loc.LocationFromSSAValue(g, v)
		av, mem = C.swapWildcardLoc(sl, mem, l)
	}

	return av, mem
}

// wrapPointers is used for implementing ssa.FieldAddr and ssa.IndexAddr.
func (C AnalysisCtxt) wrapPointers(sl defs.Superloc, g defs.Goro, mem L.Memory, ssaPtr ssa.Value, field int) (
	ret L.AbstractValue, newMem L.Memory, hasNil bool,
) {
	entries := []loc.Location{}
	v, mem := C.swapWildcard(sl, g, mem, ssaPtr)
	base := v.PointerValue()
	C.CheckPointsTo(base)

	if base.Empty() {
		fmt.Println(v, g, mem, ssaPtr, ssaPtr.Type(), ssaPtr.Parent())
		_, ok := C.LoadRes.Pointer.Queries[ssaPtr]
		fmt.Println(ok)
		log.Fatalln("Empty points-to set for wrapPointers?", ssaPtr)
	}

	base = base.FilterNilCB(func() { hasNil = true })
	mops := L.MemOps(mem)

	for _, ptr := range base.Entries() {
		// Safety check
		if aval := mops.GetUnsafe(ptr); field != loc.AINDEX &&
			!(aval.IsStruct() || aval.IsCond()) {
			panic(fmt.Sprintf("Tried to make a field pointer to a non-struct value: %v\n"+
				"Bound to location: %s", aval, ptr))
		}

		entries = append(entries, loc.FieldLocation{Base: ptr, Index: field})
	}

	return Elements().AbstractPointer(entries), mem, hasNil
}

// Determine what are the possible successors via a single silent transition.
// Models the effect of the ssa instruction on the abstract memory.
func (s *AbsConfiguration) singleSilent(C AnalysisCtxt, g defs.Goro, cl defs.CtrLoc, initState L.AnalysisState) (
	succs L.AnalysisIntraprocess,
) {
	initMem := initState.Memory()
	defer func() {
		// Provide some additional debugging support when things fail...
		if err := recover(); err != nil {
			var typI interface{} = cl.Node()
			if n, ok := cl.Node().(*cfg.SSANode); ok {
				typI = n.Instruction()
			}

			log.Printf("Abstract interpretation of %T %s in function %s failed.\n%s\nOperands:\n", typI, cl.Node(), cl.Node().Function(), err)

			fmt.Println(StringifyNodeArguments(g, initMem, cl.Node()))

			cfg.PrintNodePosition(cl.Node(), C.LoadRes.Cfg.FileSet())

			if opts.Visualize() {
				C.LoadRes.Cfg.VisualizeFunction(cl.Node().Function())
			}

			panic(err)
		}
	}()

	//------------------------------------------------------
	//                    Helpers
	//------------------------------------------------------
	swapWildcard := func(mem L.Memory, v ssa.Value) (L.AbstractValue, L.Memory) {
		return C.swapWildcard(s.Superloc, g, mem, v)
	}
	evalSSA := func(mem L.Memory, v ssa.Value) L.AbstractValue {
		return EvaluateSSA(g, mem, v)
	}

	//------------------------------------------------------
	//				Successor dependent transfer functions
	//------------------------------------------------------
	succs = Elements().AnalysisIntraprocess()

	noop := func() {
		for succ := range cl.Successors() {
			succs = succs.Update(succ, initState)
		}
	}

	singleUpd := func(newMem L.Memory) {
		mem := initState.UpdateMemory(newMem)
		C.CheckMemory(newMem)
		succs = succs.Update(cl.Successor(), mem)
	}

	// Filter the set of successors such that we only proceed to "charged" defer calls
	filterDeferSuccessors := func() (ret []defs.CtrLoc) {
		charges, _ := initState.ThreadCharges().Get(g)
		for succ := range cl.Successors() {
			ok := true
			switch n := succ.Node().(type) {
			case *cfg.DeferCall:
				ok = charges.HasEdge(cl, succ)
			case *cfg.BuiltinCall:
				if n.IsDeferred() {
					ok = charges.HasEdge(cl, succ)
				}
			}

			if ok {
				ret = append(ret, succ)
			}
		}
		return
	}

	wrapPointers := func(ssaVal ssa.Value, field int) (L.AbstractValue, L.Memory) {
		ret, mem, hasNil := C.wrapPointers(s.Superloc, g, initMem, ssaVal, field)
		if hasNil {
			succs = succs.Update(cl.Panic(), initState)
		}
		return ret, mem
	}

	deref := func(mem L.Memory, ptr ssa.Value) (L.AbstractValue, L.Memory) {
		// Retrieve the abstract value of that location
		av, mem := swapWildcard(mem, ptr)

		res := L.Consts().BotValue()
		A.ToDeref(av).
			OnSucceed(func(av L.AbstractValue) {
				res = A.Load(av, mem)
			}).
			OnPanic(func(_ L.AbstractValue) {
				succs = succs.Update(cl.Panic(), initState)
			})

		return res, mem
	}

	updatePhiNodes := func(fromBlock, toBlock *ssa.BasicBlock) L.Memory {
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
		mem := initMem
		for _, instr := range toBlock.Instrs {
			if phi, ok := instr.(*ssa.Phi); ok {
				mem = mem.Update(
					loc.LocationFromSSAValue(g, phi),
					EvaluateSSA(g, initMem, phi.Edges[predIdx]),
				)
			} else {
				break
			}
		}

		return mem
	}

	//-----------------------------------------------------------
	//							Control location case analysis
	//-----------------------------------------------------------

	C.LogCtrLocMemory(g, cl, initMem)
	switch n := cl.Node().(type) {
	case *cfg.FunctionEntry:
		succs = succs.Update(cl.Successor(), initState)
	case *cfg.FunctionExit:
		succs = C.exitSuccs(g, cl, initState)
	case *cfg.PostCall:
		if cl.Exiting() {
			// If the exiting flag is set we should immediately begin processing deferred calls.
			// We set the return value to bottom to avoid crashes when looking it up.
			succs = succs.Update(
				cl.Derive(n.PanicCont()),
				initState.UpdateMemory(
					initMem.Update(loc.ReturnLocation(g, n.Function()), L.Consts().BotValue()),
				),
			)
		} else {
			noop()
		}

	case *cfg.BuiltinCall:
		switch n.Builtin().Name() {
		case "cap":
			// For channel capacities, we can extract it as a flat element,
			// and convert it to an abstract value.
			arg := n.Arg(0)
			if _, ok := arg.Type().Underlying().(*T.Chan); !ok {
				// Proceed normally if not operating on channels.
				singleUpd(spoofCall(g, n.Call, initMem))
				break
			}

			ch, mem := swapWildcard(initMem, arg)
			mops := L.MemOps(mem)
			res := Elements().AbstractBasic(0).ToBot()
			ch.PointerValue().ForEach(func(l loc.Location) {
				// If the result is already ⊤, stop early.
				if res.BasicValue().IsTop() {
					return
				}

				if _, isNil := l.(loc.NilLocation); isNil {
					// Nil channels have capacity 0.
					res = res.MonoJoin(Elements().AbstractBasic(int64(0)))
					return
				}

				ch := mops.GetUnsafe(l).ChanValue()
				if !ch.CapacityKnown() ||
					!C.IsLocationFocused(l, mem) {
					// If the capacity is unknown, or one of the channels may not be focused,
					// the channel's capacity is ⊤
					res = res.ToTop()
					return
				}

				// Otherwise, join all the possible capacities
				kap := ch.Capacity().FlatInt().IValue()
				res = res.MonoJoin(Elements().AbstractBasic(int64(kap)))
			})

			// Update the value of the SSA register storing the capacity.
			singleUpd(mem.Update(
				loc.LocationFromSSAValue(g, n.Call.Value()),
				res))

		case "recover":
			// Optimistically assume built-in recover calls are always performed
			// when not panicking, making the results nil. Any branching on the
			// recover pointer will resolve to the nil branch
			singleUpd(initMem.Update(
				loc.LocationFromSSAValue(g, n.Call.Value()),
				L.Elements().AbstractPointerV(loc.NilLocation{}),
			))
		case "append":
			// BaseV contains a set of pointers to possible base arrays
			slice, apps := n.Args()[0], n.Args()[1]
			v, mem := swapWildcard(initMem, slice)
			baseV := v.PointerValue()

			C.CheckPointsTo(baseV)
			var argVs L.AbstractValue
			if t, ok := apps.Type().Underlying().(*T.Basic); ok &&
				t.Info()&T.IsString != 0 {
				argVs = A.TypeAdapter(apps.Type(), slice.Type(), EvaluateSSA(g, mem, apps))
			} else {
				argVs, mem = deref(mem, apps)
			}

			mops := L.MemOps(mem)

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
				mops.HeapAlloc(allocSite,
					L.Elements().AbstractArray(
						L.ZeroValueForType(eType)))
				baseV = baseV.Add(allocSite)
			}

			// We transfer the argument values to the possible base arrays
			for _, ptr := range baseV.NonNilEntries() {
				mops.WeakUpdate(ptr, argVs)
			}

			// append returns a slice (which in our case still points to the same base array)
			singleUpd(mops.Memory().Update(
				loc.LocationFromSSAValue(g, rval),
				L.Consts().BotValue().Update(baseV),
			))

		case "ssa:wrapnilchk":
			// wrapnilchk returns ptr if non-nil, panics otherwise.
			// (For use in indirection wrappers.)
			argV := EvaluateSSA(g, initMem, n.Args()[0])
			if argV.IsWildcard() {
				singleUpd(initMem.Update(
					loc.LocationFromSSAValue(g, n.Call.Value()),
					argV,
				))
			} else {
				if ptrV := argV.PointerValue().FilterNilCB(func() {
					succs = succs.Update(cl.Panic(), initState)
				}); !ptrV.Empty() {
					singleUpd(initMem.Update(
						loc.LocationFromSSAValue(g, n.Call.Value()),
						argV.UpdatePointer(ptrV),
					))
				}
			}

		default:
			singleUpd(spoofCall(g, n.Call, initMem))
		}

	case *cfg.DeferCall:
		succs = C.callSuccs(s.Superloc, g, cl, initState)

	case *cfg.PostDeferCall:
		// For deferred calls we must filter the successors based on which defers are charged.
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, initState)
		}

	case *cfg.BlockEntry:
		noop()

	case *cfg.BlockExit:
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, initState)
		}

	case *cfg.BlockEntryDefer:
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, initState)
		}

	case *cfg.BlockExitDefer:
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, initState)
		}

	case *cfg.SelectDefer:
		for _, succ := range filterDeferSuccessors() {
			succs = succs.Update(succ, initState)
		}

	case *cfg.SSANode:
		switch insn := n.Instruction().(type) {
		case *ssa.Call:
			succs = C.callSuccs(s.Superloc, g, cl, initState)

		case *ssa.Defer:
			newState := initState
			// In programs such as this:
			// 	defer close(ch)
			// 	select {}
			// the defer operation will not have a defer link.
			if dfr := n.DeferLink(); dfr != nil {
				// Charge deferlink in both exiting and non-exiting states
				for pred := range dfr.Predecessors() {
					for _, exiting := range [...]bool{false, true} {
						from := cl.Derive(pred).WithExiting(exiting)
						newState = newState.AddCharge(g, from, from.Derive(dfr))
					}
				}
			}

			succs = succs.Update(cl.Successor(), newState)

		case ssa.Value:
			// Declared here to reduce code duplication.
			allocSite := loc.AllocationSiteLocation{
				Goro:    g,
				Context: insn.Parent(),
				Site:    insn,
			}

			mops := L.MemOps(initMem)
			res := L.Consts().BotValue()

			switch val := insn.(type) {
			case *ssa.Alloc:
				eT := val.Type().Underlying().(*T.Pointer).Elem()
				initVal := L.ZeroValueForType(eT)

				// TODO: When allocating a struct we should check whether it has
				// communication primitives that should be set to ⊤ according to
				// C.IsPrimitiveFocused.

				if val.Heap {
					mops.HeapAlloc(allocSite, initVal)
				} else {
					mops.Update(allocSite, initVal)
				}

				res = Elements().AbstractPointerV(allocSite)
			case *ssa.MakeChan:
				C.Metrics.AddChan(cl)
				capValue := EvaluateSSA(g, mops.Memory(), val.Size).BasicValue()
				// Convert from constant prop. lattice to flat int lattice
				switch {
				case capValue.IsBot():
					capValue = Lattices().FlatInt().Bot().Flat()
				case capValue.IsTop():
					capValue = Lattices().FlatInt().Top().Flat()
				default:
					capValue = Elements().FlatInt(int(capValue.Value().(int64)))
				}

				plType := insn.Type().Underlying().(*T.Chan).Elem()
				ch := makeChannelWithBotPayload(capValue, true, 0, plType)

				// If the channel is not in the set of FocusedPrimitives, allocate a Top value instead.
				if !C.IsPrimitiveFocused(insn) {
					topCh := ch.ToTop().ChanValue()
					mops = L.MemOps(
						mops.Memory().Allocate(allocSite, ch.UpdateChan(
							// Prevent the use of DroppedTop elements for
							// channels that have struct payloads.
							// We use a struct with top fields instead.
							topCh.UpdatePayload(L.TopValueForType(plType)),
						), true),
					)
					res = Elements().AbstractPointerV(allocSite)
				} else {
					res = mops.HeapAlloc(allocSite, ch)
				}
			case *ssa.UnOp:
				switch val.Op {
				case token.MUL:
					// Pointer dereference operation
					res, initMem = deref(mops.Memory(), val.X)
					mops = L.MemOps(initMem)

				case token.ARROW:
					// Receive on irrelevant channel
					res = L.TopValueForType(insn.Type())

				default:
					res = A.UnOp(evalSSA(initMem, val.X), val)
				}

			case *ssa.BinOp:
				x, smem := swapWildcard(initMem, val.X)
				y, smem := swapWildcard(smem, val.Y)
				res = A.BinOp(smem, x, y, val)

				if res.IsBot() {
					// Division / modulo by zero
					succs = succs.Update(cl.Panic(), initState)
					break
				}

				// Only accept increased memory size from wildcard swaps if we
				// gain branch precision.
				if !res.BasicValue().IsTop() {
					mops = L.MemOps(smem)
				}

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
						!evalSSA(initMem, call.Call.Args[0]).Eq(ptNil) {
						res = Elements().AbstractBasic(true)
					}
				}

			case *ssa.MakeClosure:
				// Put free variables in the struct
				bindings := make(map[any]L.Element)
				for i, value := range val.Bindings {
					bindings[i] = EvaluateSSA(g, initMem, value)
				}

				res = mops.HeapAlloc(allocSite, Elements().AbstractClosure(val.Fn, bindings))
			case *ssa.MakeSlice:
				// Used to create slices of dynamic length (and capacity).
				eType := val.Type().Underlying().(*T.Slice).Elem()
				res = mops.HeapAlloc(allocSite,
					Elements().AbstractArray(
						L.ZeroValueForType(eType)))

			case *ssa.MakeMap:
				typ := val.Type().Underlying().(*T.Map)
				kTyp, vTyp := typ.Key(), typ.Elem()

				res = mops.HeapAlloc(allocSite,
					// The current abstract of maps only separates keys and values.
					Elements().AbstractMap(
						L.ZeroValueForType(kTyp).ToBot(),
						L.ZeroValueForType(vTyp).ToBot()))

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
					base, mem := swapWildcard(initMem, val.X)
					mops = L.MemOps(mem)
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
						mapV := mops.GetUnsafe(ptr).StructValue()
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

					iter := EvaluateSSA(g, initMem, val.Iter).BasicValue()

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
					v := evalSSA(initMem, val.Iter)
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

					v, mem := swapWildcard(initMem, val.Iter)
					mops = L.MemOps(mem)
					bases := v.PointerValue()
					for _, ptr := range bases.NonNilEntries() {
						mapV := mops.GetUnsafe(ptr).StructValue()
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
				tVal := EvaluateSSA(g, initMem, val.Tuple)
				switch strukt := tVal.Struct().(type) {
				case *L.DroppedTop:
					typ := val.Tuple.Type().(*T.Tuple)
					TOP := L.TopValueForType(typ.At(val.Index).Type())
					res = TOP
				case L.InfiniteMap[any]:
					res = strukt.Get(val.Index).AbstractValue()
				default:
					panic("???")
				}

			case *ssa.ChangeType:
				res = EvaluateSSA(g, initMem, val.X)

			case *ssa.ChangeInterface:
				res = EvaluateSSA(g, initMem, val.X)

			case *ssa.Convert:
				inner := EvaluateSSA(g, initMem, val.X)
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
								return mops.HeapAlloc(allocSite, inner)
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
				res = mops.HeapAlloc(allocSite, EvaluateSSA(g, initMem, val.X))

			case *ssa.TypeAssert:
				v, mem := swapWildcard(initMem, val.X)
				mops = L.MemOps(mem)
				okV := L.Consts().BotValue()

				fbases := v.PointerValue().FilterNilCB(func() {
					okV = makeConstant(false)
				})

				isInterfaceAssert := T.IsInterface(val.AssertedType)
				filtered := fbases.Filter(func(ptr loc.Location) bool {
					site, hasSite := ptr.GetSite()
					if !hasSite {
						log.Fatalln("Pointer in TypeAssert has no site?", ptr)
					}

					makeItf, ok := site.(*ssa.MakeInterface)
					if !ok {
						log.Fatalln("Allocation site did not come from a MakeInterface instruction?")
					}

					dynType := makeItf.X.Type()

					var success bool
					if !isInterfaceAssert {
						/* If AssertedType is a concrete type, TypeAssert
						* checks whether the dynamic type in interface X is
						* equal to it, and if so, the result of the conversion
						* is a copy of the value in the interface. */

						// Should we maybe use IdenticalIgnoreTags instead?
						success = T.Identical(val.AssertedType, dynType)
					} else {
						/* If AssertedType is an interface, TypeAssert checks
						* whether the dynamic type of the interface is
						* assignable to it, and if so, the result of the
						* conversion is a copy of the interface value X. If
						* AssertedType is a superinterface of X.Type(), the
						* operation will fail iff the operand is nil. (Contrast
						* with ChangeInterface, which performs no nil-check.)
						* */

						success = T.AssignableTo(dynType, val.AssertedType)
					}

					okV = okV.MonoJoin(makeConstant(success))
					return success
				})

				if makeConstant(true).Leq(okV) {
					res = L.Consts().BotValue().Update(filtered)

					if !isInterfaceAssert {
						res = A.Load(res, mem)
					}
				}

				canFail := makeConstant(false).Leq(okV)

				// If the type assertion contains an "ok" component,
				// then the result is a pair between the value of and the result
				if val.CommaOk {
					if canFail {
						res = res.MonoJoin(L.ZeroValueForType(val.AssertedType))
					}

					// A tuple containing the result value and the ok flag
					res = Elements().AbstractStructV(res, okV)
				} else if canFail {
					// If the type assert may panic, add its panic successor.
					succs = succs.Update(cl.Panic(), initState)
				}

			case *ssa.Field:
				res = EvaluateSSA(g, initMem, val.X).StructValue().Get(val.Field).AbstractValue()

			case *ssa.FieldAddr:
				res, initMem = wrapPointers(val.X, val.Field)
				mops = L.MemOps(initMem)

				if res.PointerValue().Empty() && succs.Size() > 0 {
					// The points-to set only included nil
					res = L.Consts().BotValue()
				}

			case *ssa.IndexAddr:
				// TODO: Model out-of-bounds panic
				res, initMem = wrapPointers(val.X, loc.AINDEX)
				mops = L.MemOps(initMem)

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
					mops.HeapAlloc(allocSite, Elements().AbstractArray(
						L.TopValueForType(elemType),
					))

					// Return a wrapped pointer to the element.
					res = Elements().AbstractPointerV(loc.NewArrayElementLocation(allocSite))
				}

			case *ssa.Index:
				// TODO: Model out-of-bounds panic
				res = evalSSA(initMem, val.X).ArrayElementValue()

			case *ssa.Slice:
				res = evalSSA(initMem, val.X)

			case *ssa.SliceToArrayPointer:
				res = evalSSA(initMem, val.X)

			case *ssa.Range:
				res = evalSSA(initMem, val.X)

			case *ssa.Phi:
				// Registers of phi nodes are updated in `updatePhiNodes`.
				// This should really be a no-op, but the code below expects res
				// to have a non-bottom value, so we copy it out and re-insert it.
				res = evalSSA(initMem, val)

			default:
				log.Fatalf("Don't know how to handle %T %v", val, val)
			}

			if !res.IsBot() {
				singleUpd(mops.Memory().Update(loc.LocationFromSSAValue(g, insn), res))
			}

		case *ssa.Store:
			v, mem := swapWildcard(initMem, insn.Addr)
			mops := L.MemOps(mem)
			addr := v.PointerValue()
			if addr.Empty() {
				panic("ssa.Store: points-to set for addr was empty")
			}

			// TODO: Maybe refactor this out into absint/ops like ToDeref/Load.
			addr = addr.FilterNilCB(func() {
				succs = succs.Update(cl.Panic(), initState)
			})

			if !addr.Empty() {
				strongUpdate := mops.CanStrongUpdate(addr)

				// TODO: Is it important to do the wildcard swapping here?
				val, mem := swapWildcard(mops.Memory(), insn.Val)
				mops = L.MemOps(mem)
				for _, ptr := range addr.Entries() {
					if site, ok := ptr.GetSite(); ok {
						t1 := site.Type()
						t2 := insn.Addr.Type().Underlying().(*T.Pointer).Elem()
						mops.UpdateW(ptr, A.TypeAdapter(t1, t2, val), !strongUpdate)
					} else {
						mops.UpdateW(ptr, val, !strongUpdate)
					}
				}

				singleUpd(mops.Memory())
			}

		case *ssa.MapUpdate:
			v, mem := swapWildcard(initMem, insn.Map)
			initMem = mem
			maps := v.PointerValue()
			keyV := EvaluateSSA(g, initMem, insn.Key)
			valV := EvaluateSSA(g, initMem, insn.Value)

			maps = maps.FilterNilCB(func() {
				succs = succs.Update(cl.Panic(), initState)
			})

			if !maps.Empty() {
				mops := L.MemOps(initMem)

				for _, ptr := range maps.Entries() {
					// Merge the key and value abstract values into the map.
					mops.WeakUpdate(ptr, Elements().AbstractMap(keyV, valV))
				}

				singleUpd(mops.Memory())
			}

		case *ssa.Return:
			// Put returned value in special location
			newMem := initMem
			var rval L.AbstractValue

			if len(insn.Results) == 1 {
				rval = EvaluateSSA(g, initMem, insn.Results[0])
			} else {
				// Make tuple
				bindings := make(map[interface{}]L.Element)
				for i, res := range insn.Results {
					bindings[i] = EvaluateSSA(g, initMem, res)
				}

				rval = Elements().AbstractStruct(bindings)
			}

			rloc := loc.ReturnLocation(g, insn.Parent())
			newState := initState.UpdateMemory(newMem.Update(rloc, rval))

			// Return can have multiple successors if defers are under conditionals
			for _, succ := range filterDeferSuccessors() {
				succs = succs.Update(succ, newState)
			}

		case *ssa.Go:
			// Goroutine spawning is handled by caller.
			// TODO: Perhaps we need to do top-injection here too?
			noop()

			// Spawning a goroutine can panic if the spawned function is an
			// interface method and the receiver is nil.
			succs = succs.Update(cl.Panic(), initState)

		case *ssa.Jump:
			bl := insn.Block()
			singleUpd(updatePhiNodes(bl, bl.Succs[0]))

		case *ssa.If:
			condV := EvaluateSSA(g, initMem, insn.Cond).BasicValue()

			bl := insn.Block()
			// Hacking our way around...
		SUCCESSOR:
			for succ := range cl.Successors() {
				// Due to compression the first instruction of the successor block may not exist in the CFG.
				// We try to match the block indices instead of exact CFG nodes.
				for i, blk := range insn.Block().Succs {
					if blk == succ.Node().Block() {
						// The first successor block is used when the test is true,
						// the second is used when the test is false.
						if condV.Geq(Elements().Constant(i == 0)) {
							succs = succs.Update(succ, initState.UpdateMemory(updatePhiNodes(bl, blk)))
						}
						continue SUCCESSOR
					}
				}

				log.Fatalln("Unable to match", succ, "with an if successor block")
			}

		case *ssa.Panic:
			// TODO: The value should maybe be passed on somehow?
			succs = succs.Update(cl.Panic(), initState)

		case *ssa.Send:
			// Send on an irrelevant channel
			noop()

		default:
			panic(fmt.Sprintf("Don't know how to handle %T %v", insn, insn))
		}

	case *cfg.Select:
		// Select on irrelevant channels
		for _, op := range n.Ops() {
			ncl := cl.Derive(op.Successor())
			switch op := op.(type) {
			case *cfg.SelectRcv:
				mem := initMem
				for _, val := range []ssa.Value{op.Val, op.Ok} {
					if val != nil {
						mem = mem.Update(
							loc.LocationFromSSAValue(g, val),
							L.TopValueForType(val.Type()),
						)
					}
				}

				succs = succs.Update(ncl, initState.UpdateMemory(mem))
			default:
				succs = succs.Update(ncl, initState)
			}
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
					av := EvaluateSSA(g, initMem, v)
					log.Println("Has abstract value", av)

					if av.IsPointer() {
						mops := L.MemOps(initMem)
						for _, p := range av.PointerValue().NonNilEntries() {
							if fp, ok := p.(loc.FunctionPointer); ok {
								fmt.Println("Function pointer", fp)
							} else {
								v, _ := mops.Get(p)
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
		charges, _ := initState.ThreadCharges().Get(g)
		log.Printf("Charged edges at node: %q\n", charges.Edges(cl))
		if opts.Visualize() {
			C.LoadRes.Cfg.VisualizeFunction(cl.Node().Function())
		}
		panic("")
	}

	return
}
