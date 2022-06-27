package absint

import (
	"Goat/analysis/gotopo"
	"Goat/analysis/upfront"
	"Goat/pkgutil"
	tu "Goat/testutil"
	"Goat/utils/graph"
	"fmt"
	"go/types"
	"sort"
	"testing"

	"golang.org/x/tools/go/ssa"
)

// Run the static analysis focused on each reachable primitive.
func runFocusedPrimitiveTests(t *testing.T, loadRes tu.LoadResult) {
	// Prepare runs for all reachable primitives
	pt := loadRes.Pointer
	G := graph.FromCallGraph(pt.CallGraph, true)
	entry := pt.CallGraph.Root.Func
	ps := gotopo.GetPrimitives(entry, pt, G)

	allPrimitives := map[ssa.Value]struct{}{}
	orderedPrimitives := []ssa.Value{}

	// Map from primitives to functions in which they are used
	primitiveToUses := map[ssa.Value]map[*ssa.Function]struct{}{}

	for fun, usageInfo := range ps {
		for _, usedPrimitives := range []map[ssa.Value]struct{}{
			usageInfo.Chans(),
			usageInfo.Sync(),
		} {
			for prim := range usedPrimitives {
				if prim.Parent() != nil && pkgutil.IsLocal(prim) &&
					// Due to imprecision of the pointer analysis, allocation
					// sites for primitives can be unreachable from the root.
					loadRes.CallDAG.ComponentOf(prim.Parent()) != -1 {

					// Add to list of primitives to process if this is the first encounter
					if _, seen := allPrimitives[prim]; !seen {
						allPrimitives[prim] = struct{}{}
						orderedPrimitives = append(orderedPrimitives, prim)

						primitiveToUses[prim] = map[*ssa.Function]struct{}{}
					}

					primitiveToUses[prim][fun] = struct{}{}
				}
			}
		}
	}

	t.Logf("%d local primitives reachable from %s", len(orderedPrimitives), entry)

	// Ensure consistent ordering
	sort.Slice(orderedPrimitives, func(i, j int) bool {
		return orderedPrimitives[i].String() < orderedPrimitives[j].String()
	})

	for idx, prim := range orderedPrimitives {
		primName := prim.String()
		if _, isChannel := prim.Type().Underlying().(*types.Chan); isChannel {
			if realName, ok := upfront.ChannelNames[prim.Pos()]; ok {
				primName = realName
			}
		}

		t.Run(fmt.Sprintf("focused-%d/%s", idx, primName), func(t *testing.T) {
			funs := []interface{}{prim.Parent()}
			for fun := range primitiveToUses[prim] {
				funs = append(funs, fun)
			}

			//loweredEntry := G.DominatorTree(entry)(funs...).(*ssa.Function)
			//log.Println(loweredEntry)

			C := PrepareAI().WholeProgram(loadRes)
			C.FragmentPredicateFromPrimitives([]ssa.Value{prim}, primitiveToUses)

			StaticAnalysis(C)
		})
	}
}
