package gotopo

import (
	"go/token"
	"strings"

	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/graph"

	"golang.org/x/tools/go/ssa"
)

// Map every function to the primitives it uses
type Primitives map[*ssa.Function]*Func

// Get a summary of used primitives in each function reachable from entry in
// the provided graph. Only local (according to pkgutil.IsLocal) primitives
// that are allocated in a reachable function are included in summaries.
// Additionally returns a map from primitives to the set of functions in
// which they are used, based on the previously computed summaries.
// TODO: We can make the local requirement an option?
func GetPrimitives(
	entry *ssa.Function,
	pt *u.PointerResult,
	G graph.Graph[*ssa.Function],
	onlyChannels bool,
) (p Primitives, primsToUses map[ssa.Value]map[*ssa.Function]struct{}) {
	/* TODO: We currently lose some precision from treating every primitive
	 * inside a struct as the same primitive. I.e. with a struct such as
	 * `type S struct { mu1, mu2 sync.Mutex }`
	 * we would not (currently) be able to distinguish uses of mu1 and mu2.
	 * Requires identification of primitives both by allocation site and Path.
	 */
	p = make(Primitives)

	// Compute reachable functions first, so we can check that primitives'
	// allocation sites are reachable as we process primitive uses.
	reachable := map[*ssa.Function]bool{}
	G.BFS(entry, func(f *ssa.Function) bool {
		reachable[f] = true
		return false
	})

	G.BFS(entry, func(f *ssa.Function) bool {
		p.process(f, pt, reachable, onlyChannels)
		return false
	})

	primsToUses = map[ssa.Value]map[*ssa.Function]struct{}{}
	for fun, usageInfo := range p {
		for _, usedPrimitives := range []map[ssa.Value]struct{}{
			usageInfo.Chans(),
			usageInfo.OutChans(),
			usageInfo.Sync(),
			usageInfo.OutSync(),
		} {
			for prim := range usedPrimitives {
				if _, seen := primsToUses[prim]; !seen {
					primsToUses[prim] = make(map[*ssa.Function]struct{})
				}
				primsToUses[prim][fun] = struct{}{}
			}
		}
	}
	return
}

func (p Primitives) Chans() utils.SSAValueSet {
	set := utils.MakeSSASet()

	for _, info := range p {
		for ch := range info.Chans() {
			set = set.Add(ch)
		}
		for ch := range info.OutChans() {
			set = set.Add(ch)
		}
	}

	return set
}

type _CONCURRENT_CALL int

const (
	_NOT_CONCURRENT = iota
	_CHAN_CALL
	_SYNC_CALL
	_BLOCKING_SYNC_CALL
)

func isConcurrentCall(cc ssa.CallCommon) (ssa.Value, _CONCURRENT_CALL) {
	oneOf := func(name string, valid ...string) bool {
		for _, i := range valid {
			if i == name {
				return true
			}
		}
		return false
	}

	if len(cc.Args) == 0 {
		if utils.IsNamedType(cc.Value.Type(), "sync", "Locker") {
			switch {
			// Locker dynamically dispatched method call:
			case oneOf(cc.Method.Name(), "Lock"):
				return cc.Value, _BLOCKING_SYNC_CALL
			case oneOf(cc.Method.Name(), "Unlock"):
				return cc.Value, _SYNC_CALL
			}
		}
		return nil, _NOT_CONCURRENT
	}

	receiver := cc.Args[0]

	if bi, ok := cc.Value.(*ssa.Builtin); ok {
		if bi.Name() == "close" {
			return receiver, _CHAN_CALL
		}
	}

	if sc := cc.StaticCallee(); sc != nil {
		switch len(cc.Args) {
		case 1, 2:
			rcvrType := receiver.Type()

			if utils.IsModelledConcurrentAPIType(rcvrType) {
				switch {
				// Both WaitGroup and Cond have a blocking Wait
				case oneOf(sc.Name(), "Lock", "RLock", "Wait"):
					return receiver, _BLOCKING_SYNC_CALL
				case oneOf(sc.Name(), "Unlock", "RUnlock", "Broadcast", "Signal", "Done", "Add"):
					return receiver, _SYNC_CALL
				}
			}
		}
	}
	return nil, _NOT_CONCURRENT
}

func (p Primitives) process(
	f *ssa.Function,
	pt *u.PointerResult,
	reachableFuns map[*ssa.Function]bool,
	onlyChannels bool,
) {
	fu := newFunc()

	// Functions with no blocks are un-analyzable.
	// Optimistically assume that channel primitives are not used.
	// TODO: Sound alternative: pessimistically assume parameters are used
	if len(f.Blocks) == 0 {
		p[f] = fu
		return
	}

	addPrimitive := func(v ssa.Value, update func(ssa.Value)) {
		for p := range getPrimitives(v, pt) {
			if pkgutil.IsLocal(p) && reachableFuns[p.Parent()] {
				update(p)
			}
		}
	}

	// Add all potential parameters to the in-flow set
	// inDataflow := make(map[ssa.Value]struct{})
	// for _, p := range f.Params {
	// 	if _, ok := p.Type().Underlying().(*types.Chan); ok {
	// 		addPrimitive(p, fu.AddInChan)
	// 	}
	// 	inDataflow[p] = struct{}{}
	// }

	// Add all potential parameters to the in-flow set
	// for _, fv := range f.FreeVars {
	// 	if _, ok := fv.Type().Underlying().(*types.Chan); ok {
	// 		addPrimitive(fv, fu.AddInChan)
	// 	}
	// 	inDataflow[fv] = struct{}{}
	// }

	// First visit the make chan instructions, to ensure that all channel
	// creations are captured first.
	bbGraph := graph.FromBasicBlocks(f)
	bbGraph.BFS(0, func(blockIdx int) bool {
		block := f.Blocks[blockIdx]

		for _, i := range block.Instrs {
			if i, ok := i.(*ssa.MakeChan); ok {
				addPrimitive(i, fu.AddCreatedChan)
			}
		}

		return false
	})

	// Visit the blocks in a function such that unreachable blocks are pruned
	bbGraph.BFS(0, func(blockIdx int) bool {
		block := f.Blocks[blockIdx]

		for _, i := range block.Instrs {
			switch i := i.(type) {
			case *ssa.MakeChan:
				addPrimitive(i, fu.AddCreatedChan)
			case ssa.CallInstruction:
				p, call := isConcurrentCall(*i.Common())
				switch {
				case call == _CHAN_CALL:
					addPrimitive(p, fu.AddUseChan)

				// Only process sync calls if onlyChannels is false
				case (call == _SYNC_CALL || call == _BLOCKING_SYNC_CALL) && !onlyChannels:
					addPrimitive(p, fu.AddUseSync)
				}

				/* if val := i.Value(); val != nil {
					if _, ok := val.Type().Underlying().(*types.Chan); ok {
						addPrimitive(p, fu.AddInChan)
					}
				} */
			case *ssa.Send:
				addPrimitive(i.Chan, fu.AddUseChan)
			case *ssa.UnOp:
				if i.Op == token.ARROW {
					addPrimitive(i.X, fu.AddUseChan)
				}
			case *ssa.Select:
				for _, s := range i.States {
					addPrimitive(s.Chan, fu.AddUseChan)
				}
			case *ssa.Return:
				for _, r := range i.Results {
					addPrimitive(r, func(prim ssa.Value) {
						if _, isMkChan := prim.(*ssa.MakeChan); isMkChan {
							fu.AddOutChan(prim)
						} else if !onlyChannels && utils.AllocatesConcurrencyPrimitive(prim) {
							fu.AddOutSync(prim)
						}
					})
				}
			}
		}
		return false
	})

	p[f] = fu
}

func (p Primitives) String() string {
	strs := make([]string, 0, len(p))
	for f, fu := range p {
		fustr := fu.String()
		if fustr != "" {
			strs = append(strs, colorize.Func(f)+": [\n"+fu.String()+"]\n")
		}
	}

	return strings.Join(strs, "\n")
}
