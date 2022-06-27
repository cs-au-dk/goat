package defs

import (
	"Goat/analysis/cfg"
	u "Goat/analysis/upfront"
	"Goat/utils"
	"Goat/utils/pq"
	"fmt"
)

// A worklist that contains control locations.
// It orders elements of the list according to the ordering described in InitializeCtrLocPriorities.
// The list also uses a map of the contained elements to prevent duplicates.
func EmptyIntraprocessWorklist(prios u.CtrLocPriorities) pq.PriorityQueue[CtrLoc] {
	funPriorities, blockPriorities := prios.FunPriorities, prios.BlockPriorities

	if funPriorities == nil {
		panic("CtrLoc priorities are uninitialized!")
	}

	return pq.Empty(func(a, b CtrLoc) bool {
		fa, fb := a.Node().Function(), b.Node().Function()

		if fa == fb {
			// We use block index to compare nodes in the same function
			bprios := blockPriorities[fa]

			// Helper function to assign a block priority to nodes without a block
			getBlockPrio := func(n cfg.Node) (res int) {
				if block := n.Block(); block != nil {
					res = bprios[block.Index]
				} else {
					switch n.(type) {
					case *cfg.FunctionEntry:
						res = -1
					case *cfg.FunctionExit:
						res = len(fa.Blocks)
					default:
						panic(fmt.Errorf("No block priority implementation for: %T %v", n, n))
					}
				}

				// Deferred nodes should be processed last - and in inverse block order!
				if n.IsDeferred() {
					res = 2*len(fa.Blocks) + 1 - res
				}

				return
			}

			ab, bb := getBlockPrio(a.Node()), getBlockPrio(b.Node())
			if ab != bb {
				return ab < bb
			}

			// TODO: We should probably use instruction index here, but it's a bit complicated to get.
			phasher := utils.PointerHasher{}
			return phasher.Hash(a.Node()) < phasher.Hash(b.Node())
		} else {
			p1, f1 := funPriorities[fa]
			p2, f2 := funPriorities[fb]

			if !f1 {
				panic(fmt.Errorf("Missing priority for %s", fa))
			} else if !f2 {
				panic(fmt.Errorf("Missing priority for %s", fb))
			}

			return p1 < p2
		}
	})
}
