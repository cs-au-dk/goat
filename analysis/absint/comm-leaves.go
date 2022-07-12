package absint

import (
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"
	"Goat/utils"
	"go/token"
	"log"

	"golang.org/x/tools/go/ssa"
)

// Determine what are the possible successor locations when synchronizing for a single thread.
// Uses the points-to set as an indicator.
func (C AnalysisCtxt) computeCommunicationLeaves(g defs.Goro, state L.AnalysisState, cl defs.CtrLoc) (res map[defs.CtrLoc]struct{}, newState L.AnalysisState) {
	getPrimitives := func(v ssa.Value) L.PointsTo {
		ptSet, newState := C.getConcPrimitivesForValue(g, state, v)
		state = newState

		return ptSet
	}

	getCommSuccs := func(v ssa.Value, typ cfg.SYNTH_TYPE_ID) (
		res map[defs.CtrLoc]struct{}, newMem L.AnalysisState) {
		res = make(map[defs.CtrLoc]struct{})
		// TODO: Properly handle nil channel
		ptSet := getPrimitives(v).FilterNil()
		toSync := ptSet
		// if !C.Empty() && false { // TODO: Disabled for the time being due to the bug mentioned at `if addSuccs { ...`
		// 	toSync = ptSet.MonoMeet(C)
		// }
		// addSuccs is true if ptSet is not a subset of C
		addSuccs := toSync.Size() < ptSet.Size()
		for _, c := range toSync.Entries() {
			// Create configuration information for concrete synchronization node
			config := cfg.SynthConfig{
				Type: typ,
				Loc:  c,
			}
			switch n := cl.Node().(type) {
			case *cfg.SSANode:
				config.Insn = n.Instruction()
			case *cfg.SelectSend:
				var ok bool
				config.Insn, ok = n.Channel().(ssa.Instruction)
				if !ok {
					config.Function = n.Channel().Parent()
				}
			case *cfg.SelectRcv:
				var ok bool
				config.Insn, ok = n.Channel().(ssa.Instruction)
				if !ok {
					config.Function = n.Channel().Parent()
				}
			}
			commNode := cfg.CreateSynthetic(config)
			commNode.AddPredecessor(cl.Node())

			res[cl.Derive(commNode)] = struct{}{}
		}
		// Check whether any of the channels in the relevant points-to set are
		// buffered
		// (V): This might not be necessary anymore. The communication leaf
		// already includes the channel, and must simply check the abstract memory
		// to construct bufferred channel successors.
		// addSuccs = addSuccs || hasBufferedChans(toSync, mem)

		// Include "silent" transition if the pts-to set is not a subset of C
		if addSuccs {
			// TODO: We need to populate the ssa registers at receives with something here...
			for succ := range cl.Successors() {
				res[succ] = struct{}{}
			}
		}
		return res, state
	}
	getMuOpSuccs := func(v ssa.Value, i ssa.Instruction, funName string) (
		res map[defs.CtrLoc]struct{}, newMem L.AnalysisState) {
		res = make(map[defs.CtrLoc]struct{})
		ptSet := getPrimitives(v).FilterNil()

		for _, c := range ptSet.Entries() {
			config := cfg.SynthConfig{
				Loc:  c,
				Insn: i,
			}
			if l, ok := c.(loc.AllocationSiteLocation); ok &&
				l.Site != nil {
				if v, ok := l.Site.(ssa.CallInstruction); ok &&
					v.Common().Value.Name() == "RLocker" {

					switch funName {
					case "Lock":
						config.Type = cfg.SynthTypes.RWMU_RLOCK
					case "Unlock":
						config.Type = cfg.SynthTypes.RWMU_RUNLOCK
					}

					ptSet := state.GetUnsafe(l).PointerValue()
					C.CheckPointsTo(ptSet)
					for _, c := range ptSet.NonNilEntries() {
						config.Loc = c

						if config.Type == 0 {
							res[cl.Successor()] = struct{}{}
							continue
						}

						muNode := cfg.CreateSynthetic(config)
						muNode.AddPredecessor(cl.Node())

						res[cl.Derive(muNode)] = struct{}{}
					}

					continue
				}
			}

			switch funName {
			case "Lock":
				config.Type = cfg.SynthTypes.LOCK
			case "Unlock":
				config.Type = cfg.SynthTypes.UNLOCK
			case "RLock":
				config.Type = cfg.SynthTypes.RWMU_RLOCK
			case "RUnlock":
				config.Type = cfg.SynthTypes.RWMU_RUNLOCK
			}

			if config.Type == 0 {
				res[cl.Successor()] = struct{}{}
				continue
			}

			muNode := cfg.CreateSynthetic(config)
			muNode.AddPredecessor(cl.Node())

			res[cl.Derive(muNode)] = struct{}{}
		}

		return res, state
	}
	getCondSuccs := func(i ssa.CallInstruction, funName string) (
		res map[defs.CtrLoc]struct{}, newMem L.AnalysisState) {
		res = make(map[defs.CtrLoc]struct{})
		ptSet := getPrimitives(i.Common().Args[0])

		for _, c := range ptSet.NonNilEntries() {
			config := cfg.SynthConfig{
				Loc:  c,
				Insn: i,
			}

			switch i.Common().Value.Name() {
			case "Signal":
				config.Type = cfg.SynthTypes.COND_SIGNAL
			case "Broadcast":
				config.Type = cfg.SynthTypes.COND_BROADCAST
			case "Wait":
				config.Type = cfg.SynthTypes.COND_WAIT
			}

			if config.Type == 0 {
				res[cl.CallRelationNode()] = struct{}{}
				continue
			}

			condOp := cfg.CreateSynthetic(config)
			condOp.AddPredecessor(cl.Node())
			res[cl.Derive(condOp)] = struct{}{}
		}

		return res, state
	}

	getCallSuccs := func(i ssa.CallInstruction) (res map[defs.CtrLoc]struct{}, mem L.AnalysisState) {
		if sc := i.Common().StaticCallee(); sc != nil {
			if len(i.Common().Args) > 0 {
				rcvr := i.Common().Args[0]
				switch {
				case utils.IsNamedType(rcvr.Type(), "sync", "Mutex") ||
					utils.IsNamedType(rcvr.Type(), "sync", "RWMutex") &&
						sc.Name() != "RLocker":
					return getMuOpSuccs(rcvr, i, sc.Name())
				case utils.IsNamedType(rcvr.Type(), "sync", "Cond"):
					return getCondSuccs(i, sc.Name())
				}
			}
		} else {
			if method := i.Common().Method; method != nil &&
				(method.Name() == "Lock" || method.Name() == "Unlock") {
				return getMuOpSuccs(i.Common().Value, i, method.Name())
			}
		}
		return map[defs.CtrLoc]struct{}{cl: {}}, state
	}

	C.Metrics.AddCommOp(cl)

	switch n := cl.Node().(type) {
	case *cfg.FunctionExit:
		return map[defs.CtrLoc]struct{}{cl: {}}, state
	case *cfg.TerminateGoro:
		newState = state
		return
	case *cfg.PendingGo:
		newState = state
		return
	case *cfg.Waiting:
		res = make(map[defs.CtrLoc]struct{})

		ptSet := getPrimitives(n.CallInstruction().Common().Args[0])

		for _, c := range ptSet.NonNilEntries() {
			config := cfg.SynthConfig{
				Type: cfg.SynthTypes.COND_WAITING,
				Loc:  c,
				Insn: n.Instruction(),
			}

			waiting := cfg.CreateSynthetic(config)
			waiting.AddPredecessor(n)
			res[cl.Derive(waiting)] = struct{}{}
		}

		newState = state
		return
	case *cfg.Waking:
		res = make(map[defs.CtrLoc]struct{})

		ptSet := getPrimitives(n.CallInstruction().Common().Args[0])

		for _, c := range ptSet.NonNilEntries() {
			config := cfg.SynthConfig{
				Type: cfg.SynthTypes.COND_WAKING,
				Loc:  c,
				Insn: n.Instruction(),
			}

			waking := cfg.CreateSynthetic(config)
			waking.AddPredecessor(n)
			res[cl.Derive(waking)] = struct{}{}
		}

		return res, state
	case *cfg.APIConcBuiltinCall:
		res, newState = getCallSuccs(n.Call)
		if res != nil {
			return
		}
	case *cfg.SSANode:
		switch i := n.Instruction().(type) {
		case *ssa.If:
			return map[defs.CtrLoc]struct{}{cl: {}}, state
		case *ssa.Defer:
			// SSA defer calls should not be a communication node.
			// Them being found indicates a bug.
		case *ssa.Call:
			res, newState = getCallSuccs(i)
			if res != nil {
				return
			}
		case *ssa.UnOp:
			if i.Op == token.ARROW {
				return getCommSuccs(i.X, cfg.SynthTypes.COMM_RCV)
			}
		case *ssa.Send:
			return getCommSuccs(i.Chan, cfg.SynthTypes.COMM_SEND)
		}
	// If a concurrency operation was deferred, it will be captured at this point.
	case *cfg.DeferCall:
		res, newState = getCallSuccs(n.DeferLink().SSANode().Instruction().(ssa.CallInstruction))
		if res != nil {
			return
		}
	case *cfg.Select:
		return map[defs.CtrLoc]struct{}{cl: {}}, state
	case *cfg.SelectRcv:
		return getCommSuccs(n.Channel(), cfg.SynthTypes.COMM_RCV)
	case *cfg.SelectSend:
		return getCommSuccs(n.Channel(), cfg.SynthTypes.COMM_SEND)
	case *cfg.SelectDefault:
		return map[defs.CtrLoc]struct{}{cl: {}}, state
	// case *cfg.PostCall:
	// 	return map[defs.CtrLoc]struct{}{cl: {}}, state
	// case *cfg.PostDeferCall:
	// 	return map[defs.CtrLoc]struct{}{cl: {}}, state
	case *cfg.BuiltinCall:
		if n.Builtin().Name() == "close" {
			// Make sure wildcards get swapped:
			getPrimitives(n.Arg(0))
			return map[defs.CtrLoc]struct{}{cl: {}}, state
		}
	}
	log.Fatal("Should be unreachable. Attempted getting synchronization successor of non-communication node", cl)
	return
}
