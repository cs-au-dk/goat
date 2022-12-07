package absint

import (
	"fmt"
	"go/types"
	"sort"
	"testing"

	"github.com/cs-au-dk/goat/analysis/gotopo"
	"github.com/cs-au-dk/goat/analysis/upfront"
	tu "github.com/cs-au-dk/goat/testutil"
	"github.com/cs-au-dk/goat/utils/graph"

	"golang.org/x/tools/go/ssa"
)

// Run the static analysis focused on each reachable primitive.
func runFocusedPrimitiveTests(t *testing.T, loadRes tu.LoadResult, testFun absIntCommTestFunc) {
	// Prepare runs for all reachable primitives
	pt := loadRes.Pointer
	G := graph.FromCallGraph(pt.CallGraph, true)
	entry := pt.CallGraph.Root.Func
	_, primitiveToUses := gotopo.GetPrimitives(entry, pt, G)

	orderedPrimitives := make([]ssa.Value, 0, len(primitiveToUses))
	for prim := range primitiveToUses {
		orderedPrimitives = append(orderedPrimitives, prim)
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
			/* funs := []interface{}{prim.Parent()}
			for fun := range primitiveToUses[prim] {
				funs = append(funs, fun)
			}

			loweredEntry := G.DominatorTree(entry)(funs...).(*ssa.Function)
			log.Println(loweredEntry) */

			runTest(t, loadRes, testFun,
				func(loadRes tu.LoadResult) AnalysisCtxt {
					C := PrepareAI().WholeProgram(loadRes)
					C.FragmentPredicateFromPrimitives([]ssa.Value{prim}, primitiveToUses)
					return C
				},
			)
		})
	}
}
