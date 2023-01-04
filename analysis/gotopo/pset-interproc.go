package gotopo

import (
	"go/token"

	"github.com/cs-au-dk/goat/analysis"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/graph"
	"github.com/cs-au-dk/goat/utils/hmap"
	"github.com/cs-au-dk/goat/utils/slices"

	"golang.org/x/tools/go/ssa"
)

func GetSCCPSets(
	callDAG graph.SCCDecomposition[*ssa.Function],
	primitiveToUses map[ssa.Value]map[*ssa.Function]struct{},
	pt *u.PointerResult,
) PSets {
	psets := make(PSets, 0)

	getPrimitives := func(v ssa.Value) utils.SSAValueSet {
		set := utils.MakeSSASet()
		for v := range getPrimitives(v, pt) {
			if _, ok := primitiveToUses[v]; ok {
				set = set.Add(v)
			}
		}

		return set
	}

	collectOps := func(callTypes ..._CONCURRENT_CALL) func(*ssa.Function) utils.SSAValueSet {
		return func(fun *ssa.Function) utils.SSAValueSet {
			set := utils.MakeSSASet()
			for _, block := range fun.Blocks {
				for _, insn := range block.Instrs {
					switch insn := insn.(type) {
					case *ssa.Send:
						set = set.Join(getPrimitives(insn.Chan))
					case *ssa.UnOp:
						if insn.Op == token.ARROW {
							set = set.Join(getPrimitives(insn.X))
						}
					case ssa.CallInstruction:
						p, call := isConcurrentCall(*insn.Common())
						if _, found := slices.Find(callTypes, func(typ _CONCURRENT_CALL) bool {
							return typ == call
						}); found {
							set = set.Join(getPrimitives(p))
						}
					case *ssa.Select:
						for _, s := range insn.States {
							set = set.Join(getPrimitives(s.Chan))
						}
					}
				}
			}
			return set
		}
	}
	joinSets := func(a, b utils.SSAValueSet) utils.SSAValueSet { return a.Join(b) }
	unblocks := analysis.SCCAnalysis(
		callDAG,
		collectOps(_CHAN_CALL, _SYNC_CALL),
		joinSets,
	)

	blocks := analysis.SCCAnalysis(
		callDAG,
		collectOps(_BLOCKING_SYNC_CALL),
		joinSets,
	)

	D := map[ssa.Value]utils.SSAValueSet{}
	// Dependencies are added from p1 to p2 if an operation that unblocks p2 is
	// reachable from a blocking operation on p1, like GCatch PSets.
	// (Maybe the dependency direction is swapped, but it does not matter.)
	addDep := func(from ssa.Value, to utils.SSAValueSet) {
		d, ok := D[from]
		if !ok {
			d = utils.MakeSSASet()
		}
		D[from] = d.Join(to)
	}

	for ci, comp := range callDAG.Components {
		reachableUnblocks := unblocks[ci]
		reachableBlocks := blocks[ci]

		handleOp := func(usedPrim ssa.Value, canUnblock bool, canBlock bool) {
			getPrimitives(usedPrim).ForEach(func(prim ssa.Value) {
				if canBlock {
					addDep(prim, reachableUnblocks)
				}

				if canUnblock {
					// Due to return control flow, "reachability" is coarse and bidirectional.
					// All the "reachable" blocking operations may potentially come "before" us
					// so we have to add dependencies accordingly.
					reachableBlocks.ForEach(func(blockingPrim ssa.Value) {
						addDep(blockingPrim, utils.MakeSSASet(prim))
					})
				}
			})
		}

		for _, fun := range comp {
			for _, block := range fun.Blocks {
				for _, insn := range block.Instrs {
					switch insn := insn.(type) {
					case *ssa.Send:
						handleOp(insn.Chan, true, true)
					case *ssa.UnOp:
						if insn.Op == token.ARROW {
							handleOp(insn.X, true, true)
						}
					case ssa.CallInstruction:
						p, call := isConcurrentCall(*insn.Common())
						if call != _NOT_CONCURRENT {
							handleOp(p, call == _SYNC_CALL || call == _CHAN_CALL, call == _BLOCKING_SYNC_CALL)
						}
					case *ssa.Select:
						for _, s := range insn.States {
							handleOp(s.Chan, true, insn.Blocking)
						}
					}
				}
			}
		}
	}

	seen := hmap.NewMap[bool](utils.SSAValueSetHasher)
	for prim := range primitiveToUses {
		set := utils.MakeSSASet(prim)
		if deps, found := D[prim]; found {
			deps.ForEach(func(oprim ssa.Value) {
				if odeps, found := D[oprim]; found && odeps.Contains(prim) {
					set = set.Add(oprim)
				}
			})
		}

		if !seen.Get(set) {
			seen.Set(set, true)
			psets = append(psets, set)
		}
	}

	return psets
}
