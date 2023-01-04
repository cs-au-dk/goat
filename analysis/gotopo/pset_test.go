package gotopo

import (
	"github.com/cs-au-dk/goat/testutil"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/slices"
	"go/types"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func TestGCatchPSets(t *testing.T) {
	getPsets := func(loadRes testutil.LoadResult, entry string) PSets {
		mainPkg := loadRes.Mains[0]

		entryFun := mainPkg.Func(entry)

		G := loadRes.PrunedCallDAG.Original

		_, primsToUses := GetPrimitives(entryFun, loadRes.Pointer, G, false)

		computeDominator := G.DominatorTree(loadRes.Pointer.CallGraph.Root.Func)

		return GetGCatchPSets(
			loadRes.Cfg,
			entryFun,
			loadRes.Pointer,
			G,
			computeDominator,
			loadRes.PrunedCallDAG,
			primsToUses,
		)
	}

	t.Run("SingleMutexPSet", func(t *testing.T) {
		loadRes := testutil.LoadPackageFromSource(t, "test", primTestProg)
		psets := getPsets(loadRes, "main")

		mainFun := loadRes.Mains[0].Func("main")

		insn, ok := utils.FindSSAInstruction(mainFun, func(i ssa.Instruction) bool {
			alloc, ok := i.(*ssa.Alloc)
			return ok && alloc.Heap &&
				alloc.Type().(*types.Pointer).Elem().(*types.Named).Obj().Name() == "ProtectedInt"
		})
		if !ok {
			t.Fatal("Unable to find ProtectedInt alloc in main")
		}

		mkStruct := insn.(*ssa.Alloc)

		if psets.Get(mkStruct).Empty() {
			t.Errorf("No pset contains %v: %v", mkStruct, psets)
		}
	})

	t.Run("SelectChanDep", func(t *testing.T) {
		loadRes := testutil.LoadPackageFromSource(t, "test", `
			package main
			func main() {
				ch1, ch2 := make(chan int), make(chan int)
				select {
				case <-ch1:
				case <-ch2:
				default:
				}
			}`)

		psets := getPsets(loadRes, "main")
		if len(psets) != 1 {
			t.Errorf("Expected exactly one pset, got: %v", psets)
		} else if pset := psets[0]; pset.Size() != 2 {
			t.Errorf("Expected pset to contain both channels, was: %v", pset)
		}
	})

	t.Run("ChanChanDep", func(t *testing.T) {
		loadRes := testutil.LoadPackageFromSource(t, "test", `
			package main
			func ubool() bool
			func main() {
				ch1, ch2 := make(chan int), make(chan int)
				if ubool() {
					ch1 <- 0
					<-ch2
				} else {
					ch2 <- 0
					<-ch1
				}
			}`)

		psets := getPsets(loadRes, "main")
		if len(psets) != 1 {
			t.Errorf("Expected exactly one pset, got: %v", psets)
		} else if pset := psets[0]; pset.Size() != 2 {
			t.Errorf("Expected pset to contain both channels, was: %v", pset)
		}
	})

	t.Run("MutMutDep", func(t *testing.T) {
		loadRes := testutil.LoadPackageFromSource(t, "test", `
			package main
			import "sync"
			func ubool() bool
			func main() {
				var mu1, mu2 sync.Mutex
				if ubool() {
					mu1.Lock()
					mu2.Lock()
				} else {
					mu2.Lock()
					mu1.Lock()
				}
			}`)

		psets := getPsets(loadRes, "main")
		if len(psets) != 1 {
			t.Errorf("Expected exactly one pset, got: %v", psets)
		} else if pset := psets[0]; pset.Size() != 2 {
			t.Errorf("Expected pset to contain both mutexes, was: %v", pset)
		}
	})

	t.Run("ChanMutDep", func(t *testing.T) {
		loadRes := testutil.LoadPackageFromSource(t, "test", `
			package main
			import "sync"
			func ubool() bool
			func main() {
				var mu sync.Mutex
				ch := make(chan int)
				if ubool() {
					ch <- 10
					mu.Lock()
				} else {
					mu.Lock()
					<-ch
				}
			}`)

		psets := getPsets(loadRes, "main")
		if len(psets) != 1 {
			t.Errorf("Expected exactly one pset, got: %v", psets)
		} else if pset := psets[0]; pset.Size() != 2 {
			t.Errorf("Expected pset to contain both primitives, was: %v", pset)
		}
	})

	t.Run("ScopedChanDep", func(t *testing.T) {
		loadRes := testutil.LoadPackageFromSource(t, "test", `
			package main
			func f(ch chan int) {
				newch := make(chan int)
				select {
				case <-ch:
				case <-newch:
				}
			}
			func main() {
				ch := make(chan int)
				f(ch)
			}`)

		psets := getPsets(loadRes, "main")
		if len(psets) != 2 {
			t.Errorf("Expected two psets, got: %v", psets)
		} else {
			if _, found := slices.Find(psets, func(set utils.SSAValueSet) bool {
				return set.Size() == 2
			}); !found {
				t.Errorf("Expected to find a pset containing both primitives, got: %v", psets)
			}

			if _, found := slices.Find(psets, func(set utils.SSAValueSet) bool {
				return set.Size() == 1
			}); !found {
				t.Errorf("Expected to find a pset containing only newch due to smaller scope, got: %v", psets)
			}
		}
	})
}
