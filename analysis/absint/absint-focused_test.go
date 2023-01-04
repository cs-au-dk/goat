package absint

import (
	"fmt"
	"go/types"
	"sort"
	"testing"

	"github.com/cs-au-dk/goat/analysis/gotopo"
	L "github.com/cs-au-dk/goat/analysis/lattice"
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
	_, primitiveToUses := gotopo.GetPrimitives(entry, pt, G, true)

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

			runTest(t, loadRes,
				func(t *testing.T, C AnalysisCtxt, A L.Analysis, SG SuperlocGraph, m tu.NotesManager) {
					if C.Outcome == "" {
						C.Done()
					}

					t.Logf("SA outcome: %s", C.Outcome)

					if C.Outcome == OUTCOME_PANIC {
						t.Log(C.Error())
					}

					if testFun != nil {
						testFun(t, C, A, SG, m)
					}
				},
				func(loadRes tu.LoadResult) AnalysisCtxt {
					C := AIConfig{Metrics: true}.WholeProgram(loadRes)
					C.FragmentPredicateFromPrimitives([]ssa.Value{prim}, primitiveToUses)
					return C
				},
			)
		})
	}
}

func TestStaticAnalysisPSets(t *testing.T) {
	tests := []absIntCommTest{
		{
			"test-pset-annotation",
			`func f(ch chan int) { }

			func main() {
				ch1 := make(chan int) //@ pset

				ch2 := make(chan int)
				f(ch2)

				select { //@ releases
				case <-ch1:
				case <-ch2:
				}

				<-ch1 //@ blocks
			}`,
			BlockAnalysisTest,
		},
		{
			"top-inject-embedded-mutex",
			`import "sync"
			func inspectMutex(*sync.Mutex) {}
			func main() {
				var s struct { mu sync.Mutex } //@ pset
				s.mu.Lock()
				inspectMutex(&s.mu)
				s.mu.Lock() //@ blocks
			}`,
			BlockAnalysisTest,
		},
		{
			"sync.Locker.Lock",
			`import "sync"
			func lockit(l sync.Locker) { l.Lock() } //@ releases
			func main() {
				var mu sync.Mutex //@ pset
				lockit(&mu)
				mu.Lock() //@ blocks
			}`,
			BlockAnalysisTest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			loadRes := tu.LoadPackageFromSource(t, "testpackage", "package main\n\n"+test.content)
			nmgr := tu.MakeNotesManager(t, loadRes)

			var pset []ssa.Value
			nmgr.ForEachAnnotation(func(a tu.Annotation) {
				if a, ok := a.(tu.AnnPSet); ok {
					pset = append(pset, a.Value())
				}
			})

			if len(pset) == 0 {
				t.Fatal("The code does not contain PSet annotations")
			}

			pt := loadRes.Pointer
			G := graph.FromCallGraph(pt.CallGraph, true)
			entry := pt.CallGraph.Root.Func
			_, primitiveToUses := gotopo.GetPrimitives(entry, pt, G, false)

			for _, prim := range pset {
				if _, found := primitiveToUses[prim]; !found {
					t.Fatalf("Primitive %v does not have uses according to GetPrimitives!", prim)
				}
			}

			runTest(t, loadRes, test.fun,
				func(loadRes tu.LoadResult) AnalysisCtxt {
					C := PrepareAI().WholeProgram(loadRes)
					C.FragmentPredicateFromPrimitives(pset, primitiveToUses)
					return C
				})
		})
	}
}
