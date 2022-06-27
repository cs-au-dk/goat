package absint

import (
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
	"Goat/utils"
	"fmt"
	T "go/types"
	"log"

	"golang.org/x/tools/go/ssa"
)

func (C AnalysisCtxt) Blacklisted(callIns ssa.CallInstruction, sfun *ssa.Function) bool {
	// Spoof the call if it is has no ssa instructions (external)
	if len(sfun.Blocks) == 0 {
		return true
	}

	// Functions in the testing package perform a lot of locking that we do not
	// want to analyze.
	if pkg := sfun.Pkg; pkg != nil && pkg.Pkg.Name() == "testing" {
		// || pkgName == "net"
		return true
	}

	// Since we use special abstract values for mutexes and conds, we cannot
	// abstractly interpret the bodies of methods on those types.
	if recv := sfun.Signature.Recv(); recv != nil && !opts.SkipSync() &&
		utils.IsModelledConcurrentAPIType(recv.Type()) {
		return true
	}

	// Consult the analysis context to check if the function is included in the fragment.
	return !C.FragmentPredicate(callIns, sfun)
}

func (C AnalysisCtxt) TopInjectParams(
	callIns ssa.CallInstruction,
	g defs.Goro,
	state L.AnalysisState,
	blacklists map[*ssa.Function]struct{}) L.Memory {
	// Blacklisted functions may involve side-effects on pointer-like arguments.
	// All locations that are members of points-to sets of the arguments
	// must be top injected to over-approximate potential side effects.
	mops := L.MemOps(state.Memory())
	visited := map[loc.Location]bool{}

	sideffects := C.LoadRes.WrittenFields
	mapEffects := sideffects.MapCombinedInfo(blacklists)
	sliceEffects := sideffects.SliceCombinedInfo(blacklists)
	pointerEffects := sideffects.PointerCombinedInfo(blacklists)

	var rec func(L.AbstractValue)
	rec = func(v L.AbstractValue) {
		switch {
		case v.IsPointer():
			C.CheckPointsTo(v.PointerValue())
			v.PointerValue().ForEach(func(l loc.Location) {
				if _, isFunc := l.(loc.FunctionPointer); !(isFunc ||
					l.Equal(loc.NilLocation{}) ||
					visited[l]) {
					visited[l] = true
					av := mops.GetUnsafe(l)

					rec(av)

					// Values inside closures cannot be manipulated
					// We do not need to update ⊤ locations.
					if av.IsClosure() || L.IsTopLocation(l) {
						return
					}

					updated := false
					if site, found := l.GetSite(); found {
						switch {
						case C.FocusedPrimitives != nil && C.IsPrimitiveFocused(site):
							// NOTE (Unsound): Skip focused primitives when injecting top
							// TODO: We still recurse on channel payloads. Is this desired?
							// If we choose to not recurse, we have to implement something
							// else to be sound in the case of mutexes that are identified
							// by struct allocation sites (so we recurse on the other fields).
							return
						case av.IsPointer() && !pointerEffects(site) ||
							av.IsMap() && !mapEffects(site) ||
							av.IsArray() && !sliceEffects(site):
							// If no side effects are registered on a collection, do not
							// update the abstract value.
							updated = true
						}
					}

					// Update unknown structs
					if av.IsKnownStruct() {
						// If the site is not a pointer to a struct, proceed normally
						if ptT, ok := l.Type().Underlying().(*T.Pointer); ok {
							// If the site is not a pointer to a struct, proceed normally
							if structT, ok := ptT.Elem().Underlying().(*T.Struct); ok {
								// For every possible blacklisted function, compute which fields
								// they may write to
								isWritten := sideffects.FieldInfo(structT, blacklists)
								// If the value is a known structure, then don't update any fields that
								// may not be overwritten in the function call.
								sv := av.StructValue()
								av.ForEachField(func(i interface{}, av L.AbstractValue) {
									index, ok := i.(int)
									if ok && isWritten(index) {
										sv = sv.Update(i, av.ToTop())
									}
								})

								mops.Update(l, av.Update(sv))
								updated = true
							}
						}
					}

					if !updated {
						//log.Println(callIns, l, "from", av)
						mops.Update(l, av.ToTop())
					}
				}
			})
		case v.IsKnownStruct():
			v.ForEachField(func(i interface{}, v L.AbstractValue) {
				rec(v)
			})
		case v.IsChan():
			rec(v.ChanValue().Payload())
		case v.IsCond() && v.CondValue().IsLockerKnown():
			v.CondValue().KnownLockers().FilterNil().ForEach(func(l loc.Location) {
				TOP := mops.GetUnsafe(l).ToTop()
				mops.Update(l, TOP)
			})
		}
	}

	for _, arg := range callIns.Common().Args {
		rec(evaluateSSA(g, mops.Memory(), arg))
	}

	return mops.Memory()
}

func (C AnalysisCtxt) callSuccs(
	g defs.Goro,
	cl defs.CtrLoc,
	state L.AnalysisState) L.AnalysisIntraprocess {
	n := cl.Node()
	succs := Elements().AnalysisIntraprocess()

	var callIns ssa.CallInstruction
	switch n := n.(type) {
	case *cfg.DeferCall:
		callIns = n.DeferLink().(*cfg.SSANode).Instruction().(*ssa.Defer)

	case *cfg.SSANode:
		if call, ok := n.Instruction().(*ssa.Call); ok {
			callIns = call
		}
	}

	if callIns == nil {
		panic(fmt.Errorf("callSuccs of %T %v is not supported", n, n))
	}

	postCall := n.CallRelationNode()
	newState := state

	// A call node might miss a post-call node if the Andersen pointer analysis knows
	// that the receiver is nil. I.e. if the call is guaranteed to panic.
	if postCall != nil {
		// Add a charge for the post call site
		newState = state.AddCharges(g, cl.Derive(postCall))

		// First check for skippable method invocations
		if callIns.Common().IsInvoke() {
			mem, hasModel := C.stdInvoke(g, callIns, state.Memory())
			if hasModel {
				// Skip call relation node to avoid single-silent handling of post-call node
				succs = succs.Update(cl.CallRelationNode().Successor(), state.UpdateMemory(mem))
				return succs
			}
		}
	}

	paramTransfers, mayPanic := C.transferParams(*callIns.Common(), g, g, state.Memory())

	if mayPanic {
		succs = succs.Update(cl.Panic(), state)
	}

	if C.Metrics.Enabled() {
		calleeSet := make(map[*ssa.Function]struct{})

		for fun := range paramTransfers {
			calleeSet[fun] = struct{}{}
		}

		C.Metrics.AddCallees(callIns, calleeSet)
	}

	// TODO: Check that the callees computed by transferParams is a subset of
	// those available in the CFG.

	expandedFunctions := make(map[*ssa.Function]struct{})

	blacklists := make(map[*ssa.Function]struct{})
	for succ := range cl.Successors() {
		if _, isWaiting := succ.Node().(*cfg.Waiting); isWaiting {
			// (*sync.Cond).Wait is wired to a Waiting node instead of the
			// entry of the Wait-function, and will therefore not be in the
			// paramTransfers map. We special-case this here.

			// FIXME: Hacky workaround
			blacklists[nil] = struct{}{}
			continue
		}

		sfun := succ.Node().Function()

		newMem, found := paramTransfers[sfun]
		if !found {
			// Skip any kind of handling for calls that the abstract
			// interpreter knows cannot occur.
			continue
		}

		// If we have a "model" for the called function, use that.
		if nsuccs, hasModel := C.stdCall(g, cl, callIns, state, sfun); hasModel {
			succs = succs.MonoJoin(nsuccs)
		} else if C.Blacklisted(callIns, sfun) {
			blacklists[sfun] = struct{}{}
		} else {
			C.Metrics.ExpandFunction(sfun)
			// Clear the exiting flag when entering a function
			succs = succs.Update(succ.WithExiting(false), newState.UpdateMemory(newMem))
			expandedFunctions[sfun] = struct{}{}
		}
	}

	if len(blacklists) > 0 {
		// Spoof the call if it may be blacklisted
		// Top-inject parameters of the call to account for stateful side-effects
		mem := C.TopInjectParams(callIns, g, state, blacklists)

		succs = succs.WeakUpdate(
			cl.Derive(postCall),
			state.UpdateMemory(
				// Spoof call by top-injecting the return value location
				spoofCall(g, callIns, mem)),
		)
	}

	if C.Log.Enabled && len(expandedFunctions) > 3 {
		log.Println("Expanded", len(expandedFunctions), "at thread", g, ":", cl)
		for f := range expandedFunctions {
			fmt.Println(utils.SSAFunString(f))
		}
	}

	return succs
}

// For each possible called function, returns a memory where parameters have been moved from the
// caller into the memory of the callee.
// Uses points-to values to determine the possible called functions.
func (C AnalysisCtxt) transferParams(
	call ssa.CallCommon,
	fromG, toG defs.Goro,
	initMem L.Memory,
) (res map[*ssa.Function]L.Memory, mayPanic bool) {
	res = make(map[*ssa.Function]L.Memory)

	if _, ok := call.Value.(*ssa.Builtin); ok {
		// We are transferring parameters into a goroutine started with a direct call to a builtin:
		// go println(i)
		// This needs to be handled differently.
		// We transfer ssa register values from the first to the second goroutine.
		newMem := initMem
		for _, arg := range call.Args {
			// Skip constants, they don't need to be transferred (and they don't have a location)
			if _, ok := arg.(*ssa.Const); !ok {
				newMem = newMem.Update(
					loc.LocationFromSSAValue(toG, arg),
					evaluateSSA(fromG, initMem, arg),
				)
			}
		}

		// A bit hacky
		res[nil] = newMem
		return
	}

	// Pre-evaluate arguments
	var aArgs []L.AbstractValue
	for _, ssaVal := range call.Args {
		aArgs = append(aArgs, evaluateSSA(fromG, initMem, ssaVal))
	}

	type callTarget struct {
		closure L.InfiniteMap
		args    []L.AbstractValue
	}

	v, mem := C.swapWildcard(fromG, initMem, call.Value)
	initMem = mem
	bases := v.PointerValue().FilterNilCB(func() { mayPanic = true })
	C.CheckPointsTo(bases)
	targets := map[*ssa.Function]callTarget{}

	if call.IsInvoke() {
		prog := C.LoadRes.Prog

		for _, ptr := range bases.Entries() {
			aloc, ok := ptr.(loc.AllocationSiteLocation)
			if !ok {
				panic(fmt.Errorf("Pointer in invoke was not AllocationSiteLocation: %v %T", ptr, ptr))
			}

			var fun *ssa.Function
			if v, ok := aloc.Site.(ssa.CallInstruction); ok &&
				// Special handling for RLocker model...
				v.Common().Value.Name() == "RLocker" {

				rlockerFunc := v.Common().StaticCallee()
				typ := rlockerFunc.Pkg.Type("rlocker")
				fun = prog.LookupMethod(T.NewPointer(typ.Type()), call.Method.Pkg(), call.Method.Name())

			} else {
				makeItf, ok := aloc.Site.(*ssa.MakeInterface)
				if !ok {
					log.Fatalf("AllocationSiteLocation (%v) did not come from a MakeInterface instruction? %T",
						aloc, aloc.Site)
				}

				fun = prog.LookupMethod(makeItf.X.Type(), call.Method.Pkg(), call.Method.Name())
			}

			receiver := initMem.GetUnsafe(aloc)

			if tar, exists := targets[fun]; exists {
				// Join receiver with existing receivers
				tar.args[0] = tar.args[0].MonoJoin(receiver)
			} else {
				targets[fun] = callTarget{
					args: append([]L.AbstractValue{receiver}, aArgs...),
				}
			}
		}
	} else {
		for _, ptr := range bases.Entries() {
			switch ptr := ptr.(type) {
			case loc.FunctionPointer:
				targets[ptr.Fun] = callTarget{args: aArgs}

			case loc.AddressableLocation:
				closure := initMem.GetUnsafe(ptr)
				targets[closure.Closure()] = callTarget{
					closure.StructValue(),
					aArgs,
				}

			default:
				log.Fatalln(ptr, bases, "???")
			}
		}
	}

	if len(targets) == 0 && !mayPanic {
		panic(fmt.Errorf("no targets computed for call: %s with recv/value %s",
			call.String(),
			evaluateSSA(fromG, initMem, call.Value)))
	}

	for fun, target := range targets {
		newMem := initMem
		for i, argv := range target.args {
			newMem = newMem.Update(
				loc.LocationFromSSAValue(toG, fun.Params[i]),
				argv,
			)
		}

		if len(fun.FreeVars) != 0 {
			for i, fv := range fun.FreeVars {
				newMem = newMem.Update(
					loc.LocationFromSSAValue(toG, fv),
					target.closure.Get(i).AbstractValue(),
				)
			}
		}

		res[fun] = newMem
	}

	return
}
