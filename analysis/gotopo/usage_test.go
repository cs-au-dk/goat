package gotopo

import (
	"github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/testutil"
	"github.com/cs-au-dk/goat/utils"
	"go/types"
	"testing"

	"golang.org/x/tools/go/ssa"
)

func findMkChan(t *testing.T, f *ssa.Function) *ssa.MakeChan {
	insn, found := utils.FindSSAInstruction(f, func(insn ssa.Instruction) bool {
		_, ok := insn.(*ssa.MakeChan)
		return ok
	})
	if !found {
		t.Fatal("Unable to find MakeChan instruction in", f)
	}
	return insn.(*ssa.MakeChan)
}

var primTestProg = `
package main

import "sync"

type ProtectedInt struct {
	x int
	mu sync.Mutex
}

func (p *ProtectedInt) Inc() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.x++
}

func f(ch1 chan int, i *ProtectedInt) {
	ch2 := make(chan int, 1)
	select {
	case <-ch1:
	default:
		i.Inc()
	}

	ch2 <- 1
}

func g(x interface{}) {
	lock := x.(sync.Locker)
	defer lock.Lock()
}

func main() {
	ch1 := make(chan int)
	var i ProtectedInt
	f(ch1, &i)
	var locker sync.Locker = &i.mu
	g(locker)
}`

func TestGetPrimitives(t *testing.T) {
	loadRes := testutil.LoadPackageFromSource(t, "test", primTestProg)

	mainPkg := loadRes.Mains[0]

	mainFun := mainPkg.Func("main")
	mkCh1 := findMkChan(t, mainFun)
	fFun := mainPkg.Func("f")
	mkCh2 := findMkChan(t, fFun)

	G := loadRes.PrunedCallDAG.Original

	ps, primsToUses := GetPrimitives(fFun, loadRes.Pointer, G, true)

	info := ps[fFun]
	if info == nil || !info.HasChan(mkCh2) {
		t.Errorf("%v is unused in %v?", mkCh2, fFun)
	}

	if uses, ok := primsToUses[mkCh2]; !ok || len(primsToUses) != 1 {
		t.Errorf(
			"primsToUses (%v) includes primitives allocated in functions unreachable from %v",
			primsToUses, fFun)
	} else if _, found := uses[fFun]; !found || len(uses) != 1 {
		t.Errorf(
			"%v not exclusively used in %v? (%v)",
			mkCh2, fFun, uses)
	}

	ps, primsToUses = GetPrimitives(mainFun, loadRes.Pointer, G, true)

	checkChUse := func() {
		for _, ch := range [...]*ssa.MakeChan{mkCh1, mkCh2} {
			if uses, ok := primsToUses[ch]; !ok || len(uses) == 0 {
				t.Errorf("%v unexpectedly has no uses", upfront.ChannelNames[ch.Pos()])
			} else {
				if _, usedInF := uses[fFun]; !usedInF {
					t.Errorf("Expected %v to be used in %v", ch, fFun)
				}

				if len(uses) != 1 {
					t.Errorf("Expected %v to exclusively be used in %v", ch, fFun)
				}
			}
		}
	}

	checkChUse()

	if len(ps[mainFun].Chans()) != 0 {
		t.Error("main should have no chan uses")
	}

	if len(primsToUses) != 2 {
		t.Errorf("We should only collect uses for channels. Got %v", primsToUses)
	}

	ps, primsToUses = GetPrimitives(mainFun, loadRes.Pointer, G, false)

	checkChUse()

	insn, ok := utils.FindSSAInstruction(mainFun, func(i ssa.Instruction) bool {
		alloc, ok := i.(*ssa.Alloc)
		return ok && alloc.Heap &&
			alloc.Type().(*types.Pointer).Elem().(*types.Named).Obj().Name() == "ProtectedInt"
	})
	if !ok {
		t.Fatal("Unable to find ProtectedInt alloc in main")
	}

	mkStruct := insn.(*ssa.Alloc)
	IncFun := loadRes.Prog.LookupMethod(
		types.NewPointer(mainPkg.Type("ProtectedInt").Type()),
		mainPkg.Pkg, "Inc")

	if uses, found := primsToUses[mkStruct]; !found || len(uses) != 2 {
		t.Errorf("Expected %v to have exactly two uses", mkStruct)
	} else {
		for _, fun := range [...]*ssa.Function{IncFun, mainPkg.Func("g")} {
			if _, found := uses[fun]; !found {
				t.Errorf("Expected mutex to be used in %v (%v)", IncFun, uses)
			}

			if _, found := ps[fun].Sync()[mkStruct]; !found {
				t.Error("Forward/backward mismatch?")
			}
		}
	}

	if len(ps[IncFun].Chans()) != 0 {
		t.Error("Inc shouldn't use channels?")
	}
}

func TestGetPrimitivesOutFlow(t *testing.T) {
	loadRes := testutil.LoadPackageFromSource(t, "test", `
	package main

	import "sync"

	func id(x interface{}) interface{} { return x }

	func main() {
		id(make(chan int))
		var mu sync.Mutex
		id(&mu)
	}`)

	mainPkg := loadRes.Mains[0]

	mainFun := mainPkg.Func("main")
	idFun := mainPkg.Func("id")
	mkCh := findMkChan(t, mainFun)

	G := loadRes.PrunedCallDAG.Original

	ps, primsToUses := GetPrimitives(mainFun, loadRes.Pointer, G, false)

	uses, ok := ps[idFun]
	if !ok {
		t.Fatalf("No uses recorded in %v? %v", idFun, ps)
	}

	if !uses.HasChan(mkCh) {
		t.Errorf("Expected %v to be in %v", mkCh, idFun)
	} else {
		if _, found := uses.OutChans()[mkCh]; !found {
			t.Errorf("Expected %v to flow out of %v", mkCh, idFun)
		}

		if _, found := uses.Chans()[mkCh]; found {
			t.Errorf("Did not expect %v to be used in %v", mkCh, idFun)
		}
	}

	insn, ok := utils.FindSSAInstruction(mainFun, func(i ssa.Instruction) bool {
		alloc, ok := i.(*ssa.Alloc)
		return ok && alloc.Heap && utils.IsNamedType(alloc.Type(), "sync", "Mutex")
	})
	if !ok {
		t.Fatal("Unable to find Mutex alloc in main")
	}


	mkMutex := insn.(*ssa.Alloc)
	if _, found := uses.Sync()[mkMutex]; found {
		t.Errorf("Did not expect %v to be used in %v", mkMutex, idFun)
	}

	if _, found := uses.OutSync()[mkMutex]; !found {
		t.Errorf("Expected %v to flow out of %v", mkMutex, idFun)
	}

	muUses := primsToUses[mkMutex]
	if _, found := muUses[idFun]; !found {
		t.Errorf("Expected %v to be in reverse mapping for %v", idFun, mkMutex)
	}
}
