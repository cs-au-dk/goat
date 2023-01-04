package absint

import (
	"go/token"
	"log"

	"github.com/cs-au-dk/goat/analysis/absint/leaf"
	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

// getCommunicationLeaves determines what are the possible successor locations when synchronizing
// for a single thread, given the superlocation, abstract memory and current control location.
// Uses the points-to set as an indicator.
func (C AnalysisCtxt) getCommunicationLeaves(sl defs.Superloc, g defs.Goro, mem L.Memory, cl defs.CtrLoc) (res map[defs.CtrLoc]struct{}, newMem L.Memory) {
	// Create stateful wrapper around memory.
	mops := L.MemOps(mem)

	// getPrimitives retrieves a points-to set of concurrent primitives.
	// It is used for determining the points-to sets of concurrent operands.
	getPrimitives := func(v ssa.Value) L.PointsTo {
		ptSet, mem := C.getConcPrimitivesForValue(sl, g, mops.Memory(), v)
		mops = L.MemOps(mem)

		return ptSet
	}

	// getCommSuccs computes a set of communication leaves resulting from potential channel communication operations.
	getCommSuccs := func(v ssa.Value, typ cfg.SYNTH_TYPE_ID) (res map[defs.CtrLoc]struct{}, newMem L.Memory) {
		res = make(map[defs.CtrLoc]struct{})
		toSync := getPrimitives(v).FilterNil()
		for _, c := range toSync.Entries() {
			// Create configuration information for concrete synchronization node
			config := cfg.SynthConfig{Type: typ}
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
			commNode := leaf.CreateLeaf(config, c)
			commNode.AddPredecessor(cl.Node())

			res[cl.Derive(commNode)] = struct{}{}
		}
		return res, mops.Memory()
	}
	// getMuOpSuccs computes a set of communication leaves resulting from potential lock operations.
	getMuOpSuccs := func(v ssa.Value, i ssa.Instruction, funName string) (
		res map[defs.CtrLoc]struct{}, newMem L.Memory) {
		res = make(map[defs.CtrLoc]struct{})
		ptSet := getPrimitives(v).FilterNil()

		for _, c := range ptSet.Entries() {
			usedLoc := c
			config := cfg.SynthConfig{
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

					ptSet := mops.GetUnsafe(l).PointerValue()
					C.CheckPointsTo(ptSet)
					for _, c := range ptSet.NonNilEntries() {
						usedLoc = c

						if config.Type == 0 {
							res[cl.Successor()] = struct{}{}
							continue
						}

						muNode := leaf.CreateLeaf(config, usedLoc)
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

			muNode := leaf.CreateLeaf(config, usedLoc)
			muNode.AddPredecessor(cl.Node())

			res[cl.Derive(muNode)] = struct{}{}
		}

		return res, mops.Memory()
	}
	// getCondSuccs computes a set of communication leaves resulting from potential conditional variable operations.
	getCondSuccs := func(i ssa.CallInstruction) (
		res map[defs.CtrLoc]struct{}, newMem L.Memory) {
		res = make(map[defs.CtrLoc]struct{})
		ptSet := getPrimitives(i.Common().Args[0])

		for _, c := range ptSet.NonNilEntries() {
			config := cfg.SynthConfig{Insn: i}

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

			condOp := leaf.CreateLeaf(config, c)
			condOp.AddPredecessor(cl.Node())
			res[cl.Derive(condOp)] = struct{}{}
		}

		return res, mops.Memory()
	}
	getWaitGroupSuccs := func(i ssa.CallInstruction) (
		res map[defs.CtrLoc]struct{}, newMem L.Memory) {
		res = make(map[defs.CtrLoc]struct{})
		ptSet := getPrimitives(i.Common().Args[0])

		for _, c := range ptSet.NonNilEntries() {
			config := cfg.SynthConfig{Insn: i}

			var delta L.AbstractValue
			switch i.Common().Value.Name() {
			case "Add":
				config.Type = cfg.SynthTypes.WAITGROUP_ADD
				delta = EvaluateSSA(g, mops.Memory(), i.Common().Args[1])
			case "Done":
				config.Type = cfg.SynthTypes.WAITGROUP_ADD
				delta = L.Elements().AbstractBasic(int64(-1))
			case "Wait":
				config.Type = cfg.SynthTypes.WAITGROUP_WAIT
			}

			if config.Type == 0 {
				res[cl.CallRelationNode()] = struct{}{}
				continue
			}

			waitGroupOp := leaf.CreateLeaf(config, c)
			waitGroupOp.AddPredecessor(cl.Node())
			if config.Type == cfg.SynthTypes.WAITGROUP_ADD {
				waitGroupOp.(*leaf.WaitGroupAdd).Delta = delta
			}
			res[cl.Derive(waitGroupOp)] = struct{}{}
		}

		return res, mops.Memory()
	}

	// getCallSuccs computes a set of communication leaves resulting from function calls
	// and method invocations relevant to concurrency.
	getCallSuccs := func(i ssa.CallInstruction) (res map[defs.CtrLoc]struct{}, mem L.Memory) {
		if sc := i.Common().StaticCallee(); sc != nil {
			rcvr := i.Common().Args[0]
			switch {
			case utils.IsNamedType(rcvr.Type(), "sync", "Mutex") ||
				utils.IsNamedType(rcvr.Type(), "sync", "RWMutex"):
				return getMuOpSuccs(rcvr, i, sc.Name())
			case utils.IsNamedType(rcvr.Type(), "sync", "Cond"):
				return getCondSuccs(i)
			case utils.IsNamedType(rcvr.Type(), "sync", "WaitGroup"):
				return getWaitGroupSuccs(i)
			}
		} else {
			return getMuOpSuccs(i.Common().Value, i, i.Common().Method.Name())
		}
		return
	}

	C.Metrics.AddCommOp(cl)

	switch n := cl.Node().(type) {
	case *cfg.TerminateGoro:
		newMem = mops.Memory()
		return
	case *cfg.PendingGo:
		newMem = mops.Memory()
		return
	case *cfg.Waiting:
		res = make(map[defs.CtrLoc]struct{})

		ptSet := getPrimitives(n.CallInstruction().Common().Args[0])

		for _, c := range ptSet.NonNilEntries() {
			config := cfg.SynthConfig{
				Type: cfg.SynthTypes.COND_WAITING,
				Insn: n.Instruction(),
			}

			waiting := leaf.CreateLeaf(config, c)
			waiting.AddPredecessor(n)
			res[cl.Derive(waiting)] = struct{}{}
		}

		newMem = mops.Memory()
		return
	case *cfg.Waking:
		res = make(map[defs.CtrLoc]struct{})

		ptSet := getPrimitives(n.CallInstruction().Common().Args[0])

		for _, c := range ptSet.NonNilEntries() {
			config := cfg.SynthConfig{
				Type: cfg.SynthTypes.COND_WAKING,
				Insn: n.Instruction(),
			}

			waking := leaf.CreateLeaf(config, c)
			waking.AddPredecessor(n)
			res[cl.Derive(waking)] = struct{}{}
		}

		return res, mops.Memory()
	case *cfg.APIConcBuiltinCall:
		res, newMem = getCallSuccs(n.Call)
		if res != nil {
			return
		}
	case *cfg.SSANode:
		switch i := n.Instruction().(type) {
		case *ssa.Defer:
			// SSA defer calls should not be a communication node.
			// Them being found indicates a bug.
		case *ssa.Call:
			res, newMem = getCallSuccs(i)
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
		res, newMem = getCallSuccs(n.DeferLink().SSANode().Instruction().(ssa.CallInstruction))
		if res != nil {
			return
		}
	case *cfg.SelectRcv:
		return getCommSuccs(n.Channel(), cfg.SynthTypes.COMM_RCV)
	case *cfg.SelectSend:
		return getCommSuccs(n.Channel(), cfg.SynthTypes.COMM_SEND)
	case *cfg.SelectDefault:
		return cl.DeriveBatch(n.Successors()), mops.Memory()
	case *cfg.BuiltinCall:
		// Can be `close`, or `len` on a channel.
		if n.IsCommunicationNode() {
			// Make sure wildcards get swapped:
			getPrimitives(n.Arg(0))
			return map[defs.CtrLoc]struct{}{cl: {}}, mops.Memory()
		}
	}
	log.Fatal("Should be unreachable. Attempted getting synchronization successor of non-communication node", cl)
	return
}
