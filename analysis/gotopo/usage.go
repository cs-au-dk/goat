package gotopo

import (
	"Goat/utils"
	"Goat/utils/graph"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// Map every function to the primitives it uses
type Primitives map[*ssa.Function]*Func

// Process functions based on reachability
func GetPrimitives(entry *ssa.Function, pt *pointer.Result, G graph.Graph[*ssa.Function]) (p Primitives) {
	/* TODO: We currently lose some precision from treating every primitive
	 * inside a struct as the same primitive. I.e. with a struct such as
	 * `type S struct { mu1, mu2 sync.Mutex }`
	 * we would not (currently) be able to distinguish uses of mu1 and mu2.
	 * Requires identification of primitives both by allocation site and Path.
	 */
	p = make(Primitives)

	G.BFS(entry, func(f *ssa.Function) bool {
		p.process(f, pt)
		return false
	})

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

type _CONCURRENT_CALL = int

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
		case 1:
			rcvrType := receiver.Type()

			if utils.IsNamedType(rcvrType, "sync", "Mutex") ||
				utils.IsNamedType(rcvrType, "sync", "RWMutex") ||
				utils.IsNamedType(rcvrType, "sync", "Cond") {
				switch {
				// Mutex method call:
				case oneOf(sc.Name(), "Lock", "RLock", "Wait"):
					return receiver, _BLOCKING_SYNC_CALL
				// RWMutex method call:
				case oneOf(sc.Name(), "Unlock", "RUnlock", "Broadcast", "Signal"):
					return receiver, _SYNC_CALL
				}
			}
		}
	}
	return nil, _NOT_CONCURRENT
}

func (p Primitives) process(f *ssa.Function, pt *pointer.Result) {
	fu := newFunc()

	// Functions with no blocks are un-analyzable.
	// Optimistically assume that channel primitives are not used.
	// TODO: Sound alternative: pessimistically assume parameters are used
	if len(f.Blocks) == 0 {
		p[f] = fu
		return
	}

	// inDataflow := make(map[ssa.Value]struct{})

	addPrimitive := func(v ssa.Value, update func(ssa.Value)) {
		for p := range getPrimitives(v, pt) {
			update(p)
		}
	}

	// Add all potential parameters to the in-flow set
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
				// case call == _SYNC_CALL || call == _BLOCKING_SYNC_CALL:
				// 	addPrimitive(p, fu.AddUseSync)
				}

				// if val := i.Value(); val != nil {
				// 	if _, ok := val.Type().Underlying().(*types.Chan); ok {
				// 		addPrimitive(p, fu.AddInChan)
				// 	}
				// }
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
					if _, ok := r.Type().Underlying().(*types.Chan); ok {
						addPrimitive(r, fu.AddOutChan)
					}
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
