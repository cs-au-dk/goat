package absint

import (
	A "Goat/analysis/absint/ops"
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
	T "Goat/analysis/transition"
	"Goat/utils/set"
	"go/types"
	"log"

	"golang.org/x/tools/go/ssa"
)

func (s *AbsConfiguration) GetCommSuccessors(
	leaves map[defs.Goro]map[defs.CtrLoc]struct{},
	state L.AnalysisState,
) (S transfers) {

	mem := state.Memory()
	S = make(transfers)

	updMem := func(succ Successor, newMem L.Memory) {
		S.succUpdate(succ, state.UpdateMemory(newMem))
	}

	simpleLeaf := func(tid1 defs.Goro, c1 defs.CtrLoc) {
		// NOTE: The exit of function main has no successors,
		// but it shouldn't be a direct successor to a synchronization
		// operation (the latter should be followed by at least a "return" or "panic").
		if len(c1.Successors()) > 0 {
			//if !s.Threads()[tid1].Node.IsCommunicationNode() {
			S.succUpdate(Successor{
				s.Copy().DeriveThread(tid1, c1),
				T.In{Progressed: tid1},
			}, state)
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
	attemptValueRefine := func(g defs.Goro, mem L.Memory, val ssa.Value, av L.AbstractValue) L.Memory {
		switch val.(type) {
		case *ssa.Global:
			return mem
		case *ssa.Const:
			return mem
		}

		return mem.Update(loc.LocationFromSSAValue(g, val), av)
	}

	mutexOutcomeUpdate := func(
		cl defs.CtrLoc,
		t T.Transition,
		opReg ssa.Value,
		muLoc loc.Location,
		g defs.Goro,
		mem L.Memory) func(L.AbstractValue) {
		return func(av L.AbstractValue) {
			newMem := L.MemOps(mem).Update(muLoc, av).Memory()
			// Making an interface value contain a direct pointer to a mutex is invalid.
			// (It should point to the allocated interface value, which points to the mutex.)
			if _, isItf := opReg.Type().Underlying().(*types.Interface); !isItf {
				newMem = attemptValueRefine(g, newMem, opReg, Elements().AbstractPointerV(muLoc))
			}

			updMem(Successor{s.Copy().DeriveThread(g, cl), t}, newMem)
		}
	}

	for g1 := range leaves {
		for c1 := range leaves[g1] {
			switch n1 := c1.Node().(type) {
			// A Cond value wakes some goroutine
			case *cfg.CondWaking:
				mops := L.MemOps(mem)
				condVal := mops.GetUnsafe(n1.Cnd)
				// Reset locker points-to set
				freshCond := Elements().Cond()

				lockerMem := mem
				prevLockerMem := mem
				successes := 0

				// If the locker is unknown, optimistically assume
				// the goroutine may wake without blocking
				if !condVal.Cond().IsLockerKnown() {
					updMem(Successor{
						s.Copy().DeriveThread(
							g1,
							c1.Derive(n1.Predecessor().Successor())),
						T.Wake{Progressed: g1, Cond: n1.Cnd},
					}, mops.Memory())
					continue
				}

				// Overapproximate which lockers may successfully exit from the
				// .Wait call
				for _, lockerAddr := range condVal.Cond().KnownLockers().NonNilEntries() {
					lockerMem = lockerMem.MonoJoin(prevLockerMem)
					locker := mops.GetUnsafe(lockerAddr)
					switch {
					case locker.IsMutex() || locker.IsRWMutex():
						log.Fatalln("How did this happen???")
					case locker.IsPointer():
						for _, muLoc := range locker.PointerValue().NonNilEntries() {
							A.CondWake(mops.GetUnsafe(muLoc)).OnSucceed(
								func(av L.AbstractValue) {
									mumops := L.MemOps(lockerMem)
									mumops.Update(muLoc, av)
									lockerMem = mumops.Memory()
									freshCond = freshCond.AddLocker(lockerAddr)
									successes++
								})
						}
					}
				}

				if successes > 1 {
					lockerMem = lockerMem.MonoJoin(prevLockerMem)
				}

				// NOTE: Before we woke even if successes was 0, was this intended behavior?
				if successes > 0 {
					mops = L.MemOps(attemptValueRefine(g1, lockerMem, n1.Predecessor().Cond(), Elements().AbstractPointerV(n1.Cnd)))

					mops.Update(n1.Cnd, condVal.UpdateCond(freshCond))

					updMem(Successor{
						s.Copy().DeriveThread(
							g1,
							c1.Derive(n1.Predecessor().Successor())),
						T.Wake{Progressed: g1, Cond: n1.Cnd},
					}, mops.Memory())
				}
			// A Cond value wants to put some goroutine to sleep
			case *cfg.CondWait:
				newMem := mem

				// Get Cond value
				mops := L.MemOps(newMem)
				condVal := mops.GetUnsafe(n1.Cnd)
				freshSuccCond := Elements().AbstractCond()
				freshFailCond := Elements().AbstractCond()

				lockerMem := mem
				prevLockerMem := mem
				successes := 0

				// If the locker is unknown, then optimistically assume
				// blocking at Wait succeeds
				if !condVal.Cond().IsLockerKnown() {
					updMem(Successor{
						// Should step into a Waiting node
						s.Copy().DeriveThread(g1, c1.Predecessor().Successor()),
						T.Wait{Progressed: g1, Cond: n1.Cnd},
					}, mops.Memory())
					continue
				}

				if condVal.Cond().KnownLockers().HasNil() {
					freshFailCond = freshFailCond.AddPointers(loc.NilLocation{})
				}

				for _, lockerAddr := range condVal.Cond().KnownLockers().NonNilEntries() {
					lockerMem = lockerMem.MonoJoin(prevLockerMem)

					if locker := mops.GetUnsafe(lockerAddr); !locker.IsPointer() {
						// NOTE (O): I don't think we ever get into the
						// Succeed/Panic branches here, since if the interface
						// does not contain a pointer, it won't contain a mutex
						// abstract value either.
						A.CondWait(locker).OnSucceed(
							func(av L.AbstractValue) {
								log.Fatalln("I don't think this can happen?")
								mumops := L.MemOps(lockerMem)
								mumops.Update(lockerAddr, av)
								lockerMem = mumops.Memory()
								freshSuccCond = freshSuccCond.AddPointers(lockerAddr)
								successes++
							}).OnPanic(
							func(av L.AbstractValue) {
								log.Fatalln("I don't think this can happen?")
								freshFailCond = freshFailCond.AddPointers(lockerAddr)
							})
					} else {
						for _, muLoc := range locker.PointerValue().NonNilEntries() {
							A.CondWait(mops.GetUnsafe(muLoc)).OnSucceed(
								func(av L.AbstractValue) {
									mumops := L.MemOps(lockerMem)
									mumops.Update(muLoc, av)
									lockerMem = mumops.Memory()
									freshSuccCond = freshSuccCond.AddPointers(lockerAddr)
									successes++
								}).OnPanic(
								func(L.AbstractValue) {
									freshFailCond = freshFailCond.AddPointers(lockerAddr)
								})
						}
					}
				}

				if successes > 0 {
					mops := L.MemOps(attemptValueRefine(g1, lockerMem, n1.Predecessor().Cond(), Elements().AbstractPointerV(n1.Cnd)))
					mops.Update(n1.Cnd, condVal)

					updMem(Successor{
						// Should step into a Waiting node
						s.Copy().DeriveThread(g1, c1.Predecessor().Successor()),
						T.Wait{Progressed: g1, Cond: n1.Cnd},
					}, mops.Memory())
				}
				if freshFailCond.Cond().HasLockers() {
					updMem(Successor{
						// Should step into the panic continuation (tried to unlock unlocked
						// mutex)
						s.Copy().DeriveThread(g1, c1.Predecessor().Panic()),
						T.In{Progressed: g1},
					}, mem)
				}
			case *cfg.CondSignal:
				// Refines the .Signal() Cond receiver.
				refinedMem := attemptValueRefine(g1, mem, n1.Cond(), Elements().AbstractPointerV(n1.Cnd))

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
								definiteWake = !L.MemOps(mem).IsMultialloc(n1.Cnd)

								newMem := attemptValueRefine(g2, refinedMem, n2.Cond(), Elements().AbstractPointerV(n2.Cnd))

								succConf := s.Copy().DeriveThread(
									// Step over .Signal() call
									g1, c1.Predecessor().CallRelationNode(),
								).DeriveThread(
									// Step into .Wait() post call node
									g2, c2.Predecessor().Successor(),
								)

								updMem(Successor{
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
					updMem(Successor{
						s.Copy().DeriveThread(
							// Step over .Signal() call
							g1, c1.Predecessor().CallRelationNode(),
						),
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
					sl := s.Copy().DeriveThread(g1, c1.Predecessor().CallRelationNode())
					t := T.Broadcast{
						Broadcaster:  g1,
						Cond:         n1.Cnd,
						Broadcastees: make(map[defs.Goro]struct{}),
					}

					// Refine the .Broadcast() Cond receiver.
					newMem := attemptValueRefine(g1, mem, n1.Cond(), Elements().AbstractPointerV(n1.Cnd))

					for _, cand := range candidates {
						g2, c2 := cand.g, cand.cl
						newMem = attemptValueRefine(g2, newMem, c2.Node().Cond(), Elements().AbstractPointerV(n1.Cnd))

						t.Broadcastees[g2] = struct{}{}
						sl = sl.DeriveThread(g2, c2.Predecessor().Successor())
					}

					updMem(Successor{sl, t}, newMem)
				}

				// If the Cond is single-allocated, then it is guaranteed that
				// every goroutine waiting on the same cond will progress.
				if !L.MemOps(mem).IsMultialloc(n1.Cnd) {
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
				A.Lock(L.MemOps(mem).GetUnsafe(n1.Loc)).OnSucceed(
					mutexOutcomeUpdate(
						c1.Predecessor().CallRelationNode(),
						T.Lock{Progressed: g1, Mu: n1.Loc},
						opReg, n1.Loc, g1, mem))
			case *cfg.MuUnlock:
				// Get abstract location of mutex operand
				opReg := n1.Predecessor().Locker()

				// Unlocking may either succeed or throw a fatal exception.
				A.Unlock(L.MemOps(mem).GetUnsafe(n1.Loc)).
					OnSucceed(
						mutexOutcomeUpdate(
							c1.Predecessor().CallRelationNode(),
							T.Unlock{Progressed: g1, Mu: n1.Loc},
							opReg, n1.Loc, g1, mem),
					).
					OnPanic(
						mutexOutcomeUpdate(
							c1.Predecessor().Panic(),
							T.Unlock{Progressed: g1, Mu: n1.Loc},
							opReg, n1.Loc, g1, mem))
			case *cfg.RWMuRLock:
				// Get abstract location of read mutex operand
				opReg := n1.Predecessor().Locker()

				A.RLock(L.MemOps(mem).GetUnsafe(n1.Loc)).
					OnSucceed(
						mutexOutcomeUpdate(
							c1.Predecessor().CallRelationNode(),
							T.Lock{Progressed: g1, Mu: n1.Loc},
							opReg, n1.Loc, g1, mem))
			case *cfg.RWMuRUnlock:
				// Get abstract location of read mutex operand
				opReg := n1.Predecessor().Locker()

				// Get read mutex value
				A.RUnlock(L.MemOps(mem).GetUnsafe(n1.Loc)).OnSucceed(
					mutexOutcomeUpdate(
						c1.Predecessor().CallRelationNode(),
						T.Lock{Progressed: g1, Mu: n1.Loc},
						opReg, n1.Loc, g1, mem),
				).OnPanic(
					mutexOutcomeUpdate(
						c1.Predecessor().Panic(),
						T.Lock{Progressed: g1, Mu: n1.Loc},
						opReg, n1.Loc, g1, mem))
			case *cfg.CommSend:
				// Get abstract location of the channel operand in the instruction.
				opReg1 := n1.Predecessor().Channel()

				// Retrieve sent value as an abstract value.
				sentVal := evaluateSSA(g1, mem, n1.Payload())
				// Retrieve channel value.
				chVal := mem.GetUnsafe(n1.Loc)

				// Decide on buffer representation (via configuration?)
				// outcomes := AbsFlatSend(sentVal)(chVal)
				// Compute outcomes of abstractly sending on a channel.
				outcomeUpdate := func(cl defs.CtrLoc) func(L.AbstractValue) {
					return func(val L.AbstractValue) {
						// Restrict points-to set of channel operand
						// to found channel location. If other locations are possible,
						// results will be joined at the same superlocation.
						refinedMem := attemptValueRefine(g1, mem, opReg1, Elements().AbstractPointerV(n1.Loc))
						updMem(Successor{
							s.Copy().DeriveThread(g1, cl),
							T.Send{Progressed: g1, Chan: n1.Loc},
						}, refinedMem.Update(
							// Update channel value in memory
							n1.Loc, val,
						))
					}
				}

				// The outcomes model here also cover closed synchronous channels.
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
									newMem := mem

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
												newMem = newMem.Update(
													rcvLoc,
													L.Create().Element().AbstractStructV(sentVal, okVal),
												)
											} else {
												okLoc := loc.LocationFromSSAValue(g2, ok)
												newMem = newMem.Update(
													rcvLoc,
													sentVal,
												).Update(
													okLoc,
													okVal,
												)
											}
										case val.IsChan():
											newMem = newMem.Update(
												rcvLoc,
												sentVal,
											)
										}
									}

									// Location of channel operand in the receive operation.
									opReg2 := n2.Predecessor().Channel()

									// Restrict points-to sets of channel operands to
									// determined locations. If other synchronizations are
									// possible at the same control locations, results will be joined at the superlocation.
									newMem = attemptValueRefine(g1, newMem, opReg1, Elements().AbstractPointerV(n1.Loc))
									newMem = attemptValueRefine(g2, newMem, opReg2, Elements().AbstractPointerV(n2.Loc))

									updMem(Successor{
										// SAFE: Only one predecessor per communication leaf
										// Only one successor for communication operation
										// based on the SSA IR
										s.Copy().DeriveThread(
											g1, c1.Predecessor().Successor(),
										).DeriveThread(
											g2, c2.Predecessor().Successor()),
										T.Sync{
											Channel:     n1.Loc,
											Progressed1: g1,
											Progressed2: g2,
										},
									}, newMem.Update(n1.Loc, val))
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
				chVal := mem.GetUnsafe(n1.Loc)
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
						newMem := mem
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
									newMem = newMem.Update(
										rcvLoc,
										L.Create().Element().AbstractStructV(payload, okVal),
									)
								} else {
									okLoc := loc.LocationFromSSAValue(g1, ok)
									newMem = newMem.Update(
										rcvLoc, payload,
									).Update(
										okLoc, okVal)
								}
							case val.IsChan():
								newMem = newMem.Update(rcvLoc, val.ChanValue().Payload())
							}
						}

						// Restrict points-to set of channel operand
						// to found channel location. If other locations are possible,
						// results will be joined at the superlocation.
						newMem = attemptValueRefine(g1, newMem, opReg, Elements().AbstractPointerV(n1.Loc))

						updMem(Successor{
							s.Copy().DeriveThread(g1, c1.Predecessor().Successor()),
							T.Receive{Progressed: g1, Chan: n1.Loc},
						}, newMem.Update(
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
				initVal := evaluateSSA(g1, mem, opReg)
				// Assume value is a points-to set
				ptChan := initVal.PointerValue()

				// Create a copy of memory
				newMem := mem

				// For every location in the points-to set, get the channel value.
				for _, chLoc := range ptChan.Entries() {
					outcomeUpdate := func(cl defs.CtrLoc) func(L.AbstractValue) {

						return func(val L.AbstractValue) {
							// Needed to avoid overriding newMem outside the closure
							newMem := newMem
							if chLoc, ok := chLoc.(loc.AddressableLocation); ok {
								newMem = newMem.Update(chLoc, val)
							}
							updMem(Successor{
								s.Copy().DeriveThread(g1, cl),
								T.Close{Progressed: g1, Op: n1.Arg(0)},
							},
								// The channel may be part of the operand's points-to set.
								attemptValueRefine(g1, newMem, opReg, Elements().AbstractPointerV(chLoc)),
							)
						}
					}

					var outcome L.OpOutcomes
					switch chLoc := chLoc.(type) {
					case loc.AddressableLocation:
						outcome = A.Close(mem.GetUnsafe(chLoc))
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
				S.succUpdate(Successor{
					s.Copy().DeriveThread(g1, c1),
					T.In{Progressed: g1},
				}, state)
			default:
				simpleLeaf(g1, c1)
			}
		}
	}
	return
}
