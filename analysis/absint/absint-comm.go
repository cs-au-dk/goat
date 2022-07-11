package absint

import (
	A "Goat/analysis/absint/ops"
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
	T "Goat/analysis/transition"
	"Goat/utils/set"
	"fmt"
	"go/types"
	"log"

	"golang.org/x/tools/go/ssa"
)

func (s *AbsConfiguration) GetCommSuccessors(
	C AnalysisCtxt,
	leaves map[defs.Goro]map[defs.CtrLoc]struct{},
	state L.AnalysisState,
) (S transfers) {
	tIn := func(s *AbsConfiguration, g defs.Goro) Successor {
		return Successor{s, T.In{Progressed: g}}
	}

	S = make(transfers)

	upd := func(succ Successor, newMem L.AnalysisState) {
		S.succUpdate(succ, state)
	}

	simpleLeaf := func(tid1 defs.Goro, c1 defs.CtrLoc) {
		// NOTE: The exit of function main has no successors,
		// but it shouldn't be a direct successor to a synchronization
		// operation (the latter should be followed by at least a "return" or "panic").
		if len(c1.Successors()) > 0 {
			//if !s.Threads()[tid1].Node.IsCommunicationNode() {
			tr := tIn(s.Copy().DeriveThread(tid1, c1), tid1)
			S.succUpdate(tr, state)
		}
	}

	// Used to improve the precision of receiver values after communication takes place.
	// For instance if we are trying to lock a mutex with two different pointers, but only
	// one of them is unlocked, we know that it can only be the unlocked mutex after the operation.
	// TODO: This functionality is also useful in the intraprocessual abstract interpreter.
	// For instance to rule out the nil-pointer after a successful store.
	// TODO: The method can also be made recursive such that it refines many values.
	// Here we just have to be careful not to make the analysis unsound, e.g. by refining
	// "over" previous communication operations.
	attemptValueRefine := func(g defs.Goro, state L.AnalysisState, val ssa.Value, av L.AbstractValue) L.AnalysisState {
		switch val.(type) {
		case *ssa.Global:
			return state
		case *ssa.Const:
			return state
		}

		return state.Update(loc.LocationFromSSAValue(g, val), av)
	}

	for g1 := range leaves {
		stack, heap := state.Stack(), state.Heap()

		progress1 := func(succ defs.CtrLoc) *AbsConfiguration {
			return s.Copy().DeriveThread(g1, succ)
		}
		mutexOutcomeUpdate := func(
			cl defs.CtrLoc,
			t T.Transition,
			opReg ssa.Value,
			muLoc loc.Location) func(L.AbstractValue) {
			return func(av L.AbstractValue) {
				state := state.Update(muLoc, av)
				// Making an interface value contain a direct pointer to a mutex is invalid.
				// (It should point to the allocated interface value, which points to the mutex.)
				if _, isItf := opReg.Type().Underlying().(*types.Interface); !isItf {
					state = attemptValueRefine(g1, state, opReg, Elements().AbstractPointerV(muLoc))
				}

				upd(Successor{progress1(cl), t}, state)
			}
		}

		evalSSA := func(v ssa.Value) L.AbstractValue {
			return evaluateSSA(g1, stack, v)
		}
		tIn1 := func(succ defs.CtrLoc) Successor {
			return tIn(progress1(succ), g1)
		}

		for c1 := range leaves[g1] {

			// Filter the set of successors such that we only proceed to "charged" defer calls
			filterDeferSuccessors := func() (ret []defs.CtrLoc) {
				charged, _ := state.ThreadCharges().Get(g1)
				for succ := range c1.Successors() {
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

			switch n1 := c1.Node().(type) {
			case *cfg.Select:
				// Select on irrelevant channels
				for _, op := range n1.Ops() {
					ncl := c1.Derive(op.Successor())
					newState := state
					if op, ok := op.(*cfg.SelectRcv); ok {
						for _, val := range []ssa.Value{op.Val, op.Ok} {
							if val != nil {
								newState = newState.Update(
									loc.LocationFromSSAValue(g1, val),
									L.TopValueForType(val.Type()),
								)
							}
						}
					}

					S.succUpdate(tIn1(ncl), newState)
				}

			case *cfg.DeferCall:
				C.callSuccs(g1, c1, state).ForEach(func(cl defs.CtrLoc, as L.AnalysisState) {
					S.succUpdate(
						tIn(progress1(cl), g1),
						as)
				})
			case *cfg.FunctionExit:
				anyFound := false
				// Only propagate control to charged successors.
				charged, _ := state.ThreadCharges().Get(g1)
				//callDAG := C.LoadRes.CallDAG
				retStack := L.Consts().FreshMemory()

				/* TODO
					 Abstract GC is disabled because `canGC` is expensive and because
					 it's hard to judge whether the reduced memory size outweighs the
					 benefit of not modifying the memory tree structure.
				if canGC(g, n) {
					retState = initState.UpdateMemory(abstractGC(g, n.Function(), initMem))
				}
				*/

				// NOTE: We cannot use GetUnsafe because goroutines spawned on builtins
				// get wired up as `builtin -> functionexit` (to trigger goroutine termination).
				// We instead assert that a return value exists when we actually need it.
				returnVal, hasReturnVal := stack.Get(loc.ReturnLocation(g1, n1.Function()))

				for succ := range c1.Successors() {
					// Workaround for letting init function-exit progress to main function-entry
					if sNode, isEntry := succ.Node().(*cfg.FunctionEntry); isEntry &&
						n1.Function().Name() == "init" && sNode.Function().Name() == "main" {
						anyFound = true
						S.succUpdate(tIn1(succ), state.UpdateStack(retStack))
					} else {
						for _, succExiting := range [...]bool{false, true} {
							// Check if a charged successor exists with either value of the exiting flag
							succ := succ.WithExiting(succExiting)
							if _, found := charged.Get(succ); found {
								anyFound = true

								// If the call instruction is a normal call (not defer), we need
								// to propagate the return value.
								if ssaNode, ok := succ.Node().CallRelationNode().(*cfg.SSANode); ok {
									if !hasReturnVal {
										panic(fmt.Errorf("missing return value when returning from %v", n1))
									}

									value := ssaNode.Instruction().(*ssa.Call)
									retStack.Update(loc.LocationFromSSAValue(g1, value), returnVal)
								}

								// We can remove the charged post-call node from the state if we know that the
								// function we are returning from (exiting out of) is guaranteed to not be
								// on the call stack after returning.
								// This does not remove butterfly cycles or otherwise improve precision inside
								// a single intraprocessual fixpoint computation, as we will end up joining
								// the sets of charged post-call nodes at function entry if a function is
								// called multiple times. However, if two calls to the same function are separated
								// by a communication operation in a synchronizing configuration, we will have
								// thrown away the state at function entry between the two fixpoint computations,
								// preventing the join of sets of charged post-call nodes.
								// If the two functions are in different components in the SCC convolution of the
								// call graph, the exited function cannot be on the call stack after return.
								// NOTE: This should probably be done a non-pruned (sounder) call graph.
								/* TODO: Disabled because the below safety check starting getting triggered in
										some cases. Try for instance GoKer/grpc/862. I am not sure why it happens
									and we do not have time to investigate it and fix it.
									I suspect there may be a problem with charging single nodes instead of
									charging return edges (from function exit to post-call), since this allows
									a function to return to a post-call node that never called it.
								if callDAG.ComponentOf(n.Function()) != callDAG.ComponentOf(succ.Node().Function()) {
									updatedRetState = updatedRetState.UpdateThreadCharges(
										updatedRetState.ThreadCharges().Update(g, charged.Remove(succ)),
									)
								}
								*/

								// If the exiting flag is true at FunctionExit, we should propagate
								// it no matter the value of the flag in the charged CtrLoc.
								if !succExiting && c1.Exiting() {
									succ = succ.WithExiting(true)
								}

								S.succUpdate(tIn1(succ), state.UpdateStack(retStack))
							}
						}
					}
				}

				if n1.Function() == c1.Root() {
					// If the function exit node belongs to the root function
					// of the goroutine, it indicates a potential goroutine exit point.
					S.succUpdate(tIn1(c1.Derive(
						cfg.AddSynthetic(cfg.SynthConfig{
							Type:             cfg.SynthTypes.TERMINATE_GORO,
							Function:         n1.Function(),
							TerminationCause: cfg.GoroTermination.EXIT_ROOT,
						}))), state)
				}

				// Safety check
				if !anyFound && c1.Node().Function() != c1.Root() && !c1.Panicked() {
					log.Println("Did not find any charged sites to return to?", c1)
				}
			case *cfg.SSANode:
				switch insn := n1.Instruction().(type) {
				case *ssa.Call:
					C.callSuccs(g1, c1, state).ForEach(func(cl defs.CtrLoc, as L.AnalysisState) {
						S.succUpdate(tIn1(cl), as)
					})
				case *ssa.If:

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
						stack := state.Stack()
						for _, instr := range toBlock.Instrs {
							if phi, ok := instr.(*ssa.Phi); ok {
								stack = stack.Update(
									loc.LocationFromSSAValue(g1, phi),
									evalSSA(phi.Edges[predIdx]),
								)
							} else {
								break
							}
						}

						return stack
					}

					condV := evalSSA(insn.Cond).BasicValue()

					bl := insn.Block()
					// Hacking our way around...
				SUCCESSOR:
					for succ := range c1.Successors() {
						// Due to compression the first instruction of the successor block may not exist in the CFG.
						// We try to match the block indices instead of exact CFG nodes.
						for i, blk := range insn.Block().Succs {
							if blk == succ.Node().Block() {
								// The first successor block is used when the test is true,
								// the second is used when the test is false.
								if condV.Geq(Elements().Constant(i == 0)) {
									S.succUpdate(tIn1(succ), state.UpdateStack(updatePhiNodes(bl, blk)))
								}
								continue SUCCESSOR
							}
						}

						log.Fatalln("Unable to match", succ, "with an if successor block")
					}
				}

			// A Cond value wakes some goroutine
			case *cfg.CondWaking:
				condVal := state.GetUnsafe(n1.Cnd)
				// Reset locker points-to set
				freshCond := Elements().Cond()

				lockerState := state
				prevLockerState := state
				successes := 0

				// If the locker is unknown, optimistically assume
				// the goroutine may wake without blocking
				if !condVal.Cond().IsLockerKnown() {
					upd(Successor{
						progress1(c1.Derive(n1.Predecessor().Successor())),
						T.Wake{Progressed: g1, Cond: n1.Cnd},
					}, state)
					continue
				}

				// Overapproximate which lockers may successfully exit from the
				// .Wait call
				for _, lockerAddr := range condVal.Cond().KnownLockers().NonNilEntries() {
					lockerState = lockerState.MonoJoin(prevLockerState)
					locker := state.GetUnsafe(lockerAddr)
					switch {
					case locker.IsMutex() || locker.IsRWMutex():
						log.Fatalln("How did this happen???")
					case locker.IsPointer():
						for _, muLoc := range locker.PointerValue().NonNilEntries() {
							A.CondWake(state.GetUnsafe(muLoc)).OnSucceed(
								func(av L.AbstractValue) {
									lockerState = lockerState.Update(muLoc, av)
									freshCond = freshCond.AddLocker(lockerAddr)
									successes++
								})
						}
					}
				}

				if successes > 1 {
					lockerState = lockerState.MonoJoin(prevLockerState)
				}

				// NOTE: Before we woke even if successes was 0, was this intended behavior?
				if successes > 0 {
					refine := attemptValueRefine(g1, lockerState,
						n1.Predecessor().Cond(),
						Elements().AbstractPointerV(n1.Cnd)).
						Update(n1.Cnd, condVal.UpdateCond(freshCond))

					upd(Successor{
						progress1(c1.Derive(n1.Predecessor().Successor())),
						T.Wake{Progressed: g1, Cond: n1.Cnd},
					}, refine)
				}
			// A Cond value wants to put some goroutine to sleep
			case *cfg.CondWait:
				// Get Cond value
				condVal := state.GetUnsafe(n1.Cnd)
				freshFailCond := Elements().AbstractCond()

				lockerState, prevLockerState := state, state
				successes := 0

				// If the locker is unknown, then optimistically assume
				// blocking at Wait succeeds
				if !condVal.Cond().IsLockerKnown() {
					upd(Successor{
						// Should step into a Waiting node
						progress1(c1.Predecessor().Successor()),
						T.Wait{Progressed: g1, Cond: n1.Cnd},
					}, state)
					continue
				}

				if condVal.Cond().KnownLockers().HasNil() {
					freshFailCond = freshFailCond.AddPointers(loc.NilLocation{})
				}

				for _, lockerAddr := range condVal.Cond().KnownLockers().NonNilEntries() {
					lockerState = lockerState.MonoJoin(prevLockerState)

					if locker := state.GetUnsafe(lockerAddr); !locker.IsPointer() {
						// NOTE (O): I don't think we ever get into the
						// Succeed/Panic branches here, since if the interface
						// does not contain a pointer, it won't contain a mutex
						// abstract value either.
						A.CondWait(locker).OnSucceed(
							func(av L.AbstractValue) {
								log.Fatalln("I don't think this can happen?")
								lockerState = lockerState.Update(lockerAddr, av)
								successes++
							}).OnPanic(
							func(av L.AbstractValue) {
								log.Fatalln("I don't think this can happen?")
								freshFailCond = freshFailCond.AddPointers(lockerAddr)
							})
					} else {
						for _, muLoc := range locker.PointerValue().NonNilEntries() {
							A.CondWait(state.GetUnsafe(muLoc)).OnSucceed(
								func(av L.AbstractValue) {
									lockerState = lockerState.Update(muLoc, av)
									successes++
								}).OnPanic(
								func(L.AbstractValue) {
									freshFailCond = freshFailCond.AddPointers(lockerAddr)
								})
						}
					}
				}

				if successes > 0 {
					refine := attemptValueRefine(g1, lockerState, n1.Predecessor().Cond(), Elements().AbstractPointerV(n1.Cnd)).Update(n1.Cnd, condVal)

					upd(Successor{
						// Should step into a Waiting node
						progress1(c1.Predecessor().Successor()),
						T.Wait{Progressed: g1, Cond: n1.Cnd},
					}, refine)
				}
				if freshFailCond.Cond().HasLockers() {
					// Should step into the panic continuation (tried to unlock unlocked
					// mutex)
					upd(tIn1(c1.Predecessor().Panic()), state)
				}
			case *cfg.CondSignal:
				// Refines the .Signal() Cond receiver.
				refinedMem := attemptValueRefine(g1, state, n1.Cond(), Elements().AbstractPointerV(n1.Cnd))

				// Bookmarks whether a definitely parked goroutine waiting
				// on this Cond was found.
				var definiteWake bool

				// Model waking up of some goroutine
				for g2 := range leaves {
					if g1 == g2 {
						continue
					}

					for c2 := range leaves[g2] {
						switch n2 := c2.Node().(type) {
						case *cfg.CondWaiting:
							if n1.Cnd == n2.Cnd {
								// A guaranteed partner was found only if the Cond
								// primitive is not multiallocated.
								definiteWake = !L.MemOps(heap).IsMultialloc(n1.Cnd)

								newMem := attemptValueRefine(g2, refinedMem, n2.Cond(), Elements().AbstractPointerV(n2.Cnd))

								succConf := progress1(c1.Predecessor().CallRelationNode()).DeriveThread(
									// Step into .Wait() post call node
									g2, c2.Predecessor().Successor(),
								)

								upd(Successor{
									succConf,
									T.Signal{
										Progressed1: g1, /* wakes up */
										Progressed2: g2,
										Cond:        n1.Cnd,
									},
								}, newMem)
							}
						}
					}
				}

				// If there is no guaranteed partner, model the stepping through .Signal()
				// without waking up any goroutine. An over-approximation of concurrent
				// behavior must take this into account, but could lead to many false positives.
				if !definiteWake {
					upd(Successor{
						progress1(c1.Predecessor().CallRelationNode()),
						T.Signal{
							Progressed1: g1,
							Cond:        n1.Cnd,
						},
					}, refinedMem)
				}

			case *cfg.CondBroadcast:
				// Compute all the goroutine progress candidates with their
				// associated control location.
				candidates := make(map[defs.Goro]defs.CtrLoc)

				for g2 := range leaves {
					if g1 == g2 {
						continue
					}

					for c2 := range leaves[g2] {
						switch n2 := c2.Node().(type) {
						case *cfg.CondWaiting:
							// A control location for a goroutine is a candidate if
							// it may wait on the same Cond construct
							if n1.Cnd == n2.Cnd {
								candidates[g2] = c2
							}
						}
					}
				}
				type GoroCand = struct {
					g  defs.Goro
					cl defs.CtrLoc
				}

				candList := make([]GoroCand, 0, len(candidates))

				for g, cl := range candidates {
					candList = append(candList, GoroCand{g, cl})
				}

				getSuccessor := func(candidates []GoroCand) {
					sl := progress1(c1.Predecessor().CallRelationNode())
					t := T.Broadcast{
						Broadcaster:  g1,
						Cond:         n1.Cnd,
						Broadcastees: make(map[defs.Goro]struct{}),
					}

					// Refine the .Broadcast() Cond receiver.
					newState := attemptValueRefine(g1, state, n1.Cond(), Elements().AbstractPointerV(n1.Cnd))

					for _, cand := range candidates {
						g2, c2 := cand.g, cand.cl
						newState = attemptValueRefine(g2, newState, c2.Node().Cond(), Elements().AbstractPointerV(n1.Cnd))

						t.Broadcastees[g2] = struct{}{}
						sl = sl.DeriveThread(g2, c2.Predecessor().Successor())
					}

					upd(Successor{sl, t}, newState)
				}

				// If the Cond is single-allocated, then it is guaranteed that
				// every goroutine waiting on the same cond will progress.
				if !L.MemOps(heap).IsMultialloc(n1.Cnd) {
					getSuccessor(candList)
				} else {
					// If the Cond is multi-allocated, then it is not guaranteed that all
					// candidates will be progressed along on successor. Instead, all
					// goroutine combinations must be considered.
					iCandList := make([]interface{}, 0, len(candList))
					for _, cand := range candList {
						iCandList = append(iCandList, cand)
					}

					set.Subsets(iCandList).ForEach(func(i []interface{}) {
						candList := make([]GoroCand, 0, len(i))

						for _, i := range i {
							candList = append(candList, i.(GoroCand))
						}

						getSuccessor(candList)
					})
				}

			case *cfg.MuLock:
				// Get abstract location of mutex operand
				opReg := n1.Predecessor().Locker()

				// Locking may either succeed or block.
				A.Lock(state.GetUnsafe(n1.Loc)).OnSucceed(
					mutexOutcomeUpdate(
						c1.Predecessor().CallRelationNode(),
						T.Lock{Progressed: g1, Mu: n1.Loc},
						opReg, n1.Loc))
			case *cfg.MuUnlock:
				// Get abstract location of mutex operand
				opReg := n1.Predecessor().Locker()

				// Unlocking may either succeed or throw a fatal exception.
				A.Unlock(state.GetUnsafe(n1.Loc)).
					OnSucceed(
						mutexOutcomeUpdate(
							c1.Predecessor().CallRelationNode(),
							T.Unlock{Progressed: g1, Mu: n1.Loc},
							opReg, n1.Loc),
					).
					OnPanic(
						mutexOutcomeUpdate(
							c1.Predecessor().Panic(),
							T.Unlock{Progressed: g1, Mu: n1.Loc},
							opReg, n1.Loc))
			case *cfg.RWMuRLock:
				// Get abstract location of read mutex operand
				opReg := n1.Predecessor().Locker()

				A.RLock(L.MemOps(heap).GetUnsafe(n1.Loc)).
					OnSucceed(
						mutexOutcomeUpdate(
							c1.Predecessor().CallRelationNode(),
							T.Lock{Progressed: g1, Mu: n1.Loc},
							opReg, n1.Loc))
			case *cfg.RWMuRUnlock:
				// Get abstract location of read mutex operand
				opReg := n1.Predecessor().Locker()

				// Get read mutex value
				A.RUnlock(L.MemOps(heap).GetUnsafe(n1.Loc)).OnSucceed(
					mutexOutcomeUpdate(
						c1.Predecessor().CallRelationNode(),
						T.Lock{Progressed: g1, Mu: n1.Loc},
						opReg, n1.Loc),
				).OnPanic(
					mutexOutcomeUpdate(
						c1.Predecessor().Panic(),
						T.Lock{Progressed: g1, Mu: n1.Loc},
						opReg, n1.Loc))
			case *cfg.CommSend:
				// Get abstract location of the channel operand in the instruction.
				opReg1 := n1.Predecessor().Channel()

				// Retrieve sent value as an abstract value.
				sentVal := evaluateSSA(g1, stack, n1.Payload())
				// Retrieve channel value.
				chVal := heap.GetUnsafe(n1.Loc)

				// Decide on buffer representation (via configuration?)
				// outcomes := AbsFlatSend(sentVal)(chVal)
				// Compute outcomes of abstractly sending on a channel.
				outcomeUpdate := func(cl defs.CtrLoc) func(L.AbstractValue) {
					return func(val L.AbstractValue) {
						// Restrict points-to set of channel operand
						// to found channel location. If other locations are possible,
						// results will be joined at the same superlocation.
						refinedMem := attemptValueRefine(g1, state, opReg1, Elements().AbstractPointerV(n1.Loc))
						upd(Successor{
							progress1(cl),
							T.Send{Progressed: g1, Chan: n1.Loc},
						},
							// Update channel value in memory
							refinedMem.Update(n1.Loc, val))
					}
				}

				// The outcomes modelled here also cover closed synchronous channels.
				A.IntervalSend(sentVal)(chVal).OnSucceed(
					outcomeUpdate(c1.Predecessor().Successor()),
				).OnPanic(
					outcomeUpdate(c1.Predecessor().Panic()),
				)

				// If the channel is initially at most closed, then no synchronization with a
				// partner is possible.
				if chVal.ChanValue().Status().Leq(L.Consts().Closed()) ||
					// If the channel is guaranteed not synchronous, do not model synchronizations.
					!chVal.ChanValue().MaySynchronous() {
					continue
				}

				// Find whether any synchronization options are available.
				for g2 := range leaves {
					// Avoid synchronizing on the same thread.
					if g1 == g2 {
						continue
					}

					for c2 := range leaves[g2] {
						switch n2 := c2.Node().(type) {
						case *cfg.CommRcv:
							// Since we're overapproximating possible synchronizations,
							// it's okay if the location might correspond to multiple concrete objects.
							// (I.e. ALLOC flag is ⊤.)
							if n1.Loc == n2.Loc {
								// TODO: If we know for sure that the receiver can receive something
								// from the buffer. We can gain a bit of precision by not including
								// the possibility of synchronizing with a partner.
								rcv, ok, isTuple := n2.Receiver()

								A.Sync(n2.CommaOk())(chVal).OnSucceed(func(val L.AbstractValue) {
									newState := state

									// Manage payload
									if rcv != nil {
										rcvLoc := loc.LocationFromSSAValue(g2, rcv)
										switch {
										case val.IsStruct():
											strukt := val.StructValue()
											// Fetch the channel abstract value from the struct
											val = strukt.Get(0).AbstractValue()
											okVal := strukt.Get(1).AbstractValue()
											if isTuple {
												newState = newState.Update(
													rcvLoc,
													L.Create().Element().AbstractStructV(sentVal, okVal),
												)
											} else {
												okLoc := loc.LocationFromSSAValue(g2, ok)
												newState = newState.
													Update(rcvLoc, sentVal).
													Update(okLoc, okVal)
											}
										case val.IsChan():
											newState = newState.Update(rcvLoc, sentVal)
										}
									}

									// Location of channel operand in the receive operation.
									opReg2 := n2.Predecessor().Channel()

									// Restrict points-to sets of channel operands to
									// determined locations. If other synchronizations are
									// possible at the same control locations, results will be joined at the superlocation.
									newState = attemptValueRefine(g1, newState, opReg1, Elements().AbstractPointerV(n1.Loc))
									newState = attemptValueRefine(g2, newState, opReg2, Elements().AbstractPointerV(n2.Loc))

									upd(Successor{
										// SAFE: Only one predecessor per communication leaf
										// Only one successor for communication operation
										// based on the SSA IR
										progress1(c1.Predecessor().Successor()).DeriveThread(
											g2, c2.Predecessor().Successor()),
										T.Sync{
											Channel:     n1.Loc,
											Progressed1: g1,
											Progressed2: g2,
										},
									}, newState.Update(n1.Loc, val))
								})
							}
						}
					}
				}
			case *cfg.CommRcv:
				// Get abstract location of the channel operand in the instruction.
				opReg := c1.Node().Predecessor().Channel()

				// Synchronous concrete receives only relevant if a matching send is
				// found. Only model the effects of buffered channels here, or of
				// receives on closed synchronous channels.

				// Retrieve channel value.
				chVal := state.GetUnsafe(n1.Loc)
				rcv, ok, isTuple := n1.Receiver()
				// Zero value for type.
				ZERO := Lattices().AbstractValue().Bot().AbstractValue()
				if rcv != nil {
					typ := rcv.Type()
					if isTuple {
						typ = typ.(*types.Tuple).At(0).Type()
					}

					ZERO = L.ZeroValueForType(typ)
				}

				A.IntervalReceive(ZERO, n1.CommaOk())(chVal).
					OnSucceed(func(val L.AbstractValue) {
						newState := state
						// TODO: This code seems very similar to the code for handling val in AbsSync.
						// Manage the payload.
						if rcv != nil {
							rcvLoc := loc.LocationFromSSAValue(g1, rcv)
							switch {
							case val.IsStruct():
								strukt := val.StructValue()
								// Fetch the channel abstract value from the struct
								val = strukt.Get(0).AbstractValue()
								okVal := strukt.Get(1).AbstractValue()
								payload := val.ChannelInfo().Payload()
								if isTuple {
									newState = newState.Update(
										rcvLoc,
										L.Create().Element().AbstractStructV(payload, okVal),
									)
								} else {
									okLoc := loc.LocationFromSSAValue(g1, ok)
									newState = newState.Update(
										rcvLoc, payload,
									).Update(
										okLoc, okVal)
								}
							case val.IsChan():
								newState = newState.Update(rcvLoc, val.ChanValue().Payload())
							}
						}

						// Restrict points-to set of channel operand
						// to found channel location. If other locations are possible,
						// results will be joined at the superlocation.
						newState = attemptValueRefine(g1, newState, opReg, Elements().AbstractPointerV(n1.Loc))

						upd(Successor{
							progress1(c1.Predecessor().Successor()),
							T.Receive{Progressed: g1, Chan: n1.Loc},
						}, newState.Update(
							// Update channel value in memory
							n1.Loc, val,
						))
					})

			case *cfg.BuiltinCall:
				if n1.Builtin().Name() != "close" {
					simpleLeaf(g1, c1)
					continue
				}

				if len(n1.Args()) != 1 {
					log.Fatal("Builtin call to close does not have exactly 1 argument?",
						n1.Args())
				}

				// Get SSA location for operand.
				opReg := n1.Args()[0]
				// Get abstract value of operand.
				initVal := evaluateSSA(g1, heap, opReg)
				// Assume value is a points-to set
				ptChan := initVal.PointerValue()

				// For every location in the points-to set, get the channel value.
				for _, chLoc := range ptChan.Entries() {
					outcomeUpdate := func(cl defs.CtrLoc) func(L.AbstractValue) {

						return func(val L.AbstractValue) {
							// Needed to avoid overriding newState outside the closure
							newState := state
							if chLoc, ok := chLoc.(loc.AddressableLocation); ok {
								newState = newState.Update(chLoc, val)
							}
							upd(Successor{progress1(cl), T.Close{Progressed: g1, Op: n1.Arg(0)}},
								// The channel may be part of the operand's points-to set.
								attemptValueRefine(g1, newState, opReg, Elements().AbstractPointerV(chLoc)),
							)
						}
					}

					var outcome L.OpOutcomes
					switch chLoc := chLoc.(type) {
					case loc.AddressableLocation:
						outcome = A.Close(heap.GetUnsafe(chLoc))
					case loc.NilLocation:
						outcome = A.Close(Elements().AbstractChannel())
					default:
						panic("???")
					}

					outcome.OnSucceed(
						outcomeUpdate(c1.Successor()),
					).OnPanic(
						outcomeUpdate(c1.Panic()),
					)
				}
			case *cfg.TerminateGoro:
				S.succUpdate(tIn1(c1), state)
			case *cfg.SelectDefault:
				for _, succ := range filterDeferSuccessors() {
					simpleLeaf(g1, succ)
				}
			default:
				simpleLeaf(g1, c1)
			}
		}
	}
	return
}
