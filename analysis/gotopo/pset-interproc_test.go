package gotopo

import (
	"testing"

	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/testutil"
	"golang.org/x/tools/go/ssa"
)

func TestSCCPsets(t *testing.T) {
	t.Run("CloseIsNotBlocking", func(t *testing.T) {
		loadRes := testutil.LoadPackageFromSource(t, "test", `package main
func main() {
	ch1 := make(chan int, 1)
	ch2 := make(chan struct{})
	ch1 <- 10
	close(ch2)
}
`)

		mainPkg := loadRes.Mains[0]

		entryFun := mainPkg.Func("main")

		callDAG := loadRes.PrunedCallDAG
		G := callDAG.Original

		_, primsToUses := GetPrimitives(entryFun, loadRes.Pointer, G, false)
		psets := GetSCCPSets(callDAG, primsToUses, loadRes.Pointer)
		if len(psets) == 0 {
			t.Fatal("Expected some psets to be formed")
		}

		var ch2 ssa.Value
		for prim := range primsToUses {
			if u.ChannelNames[prim.Pos()] == "main.ch2" {
				ch2 = prim
			}
		}

		if ch2 == nil {
			t.Fatal("Unable to find allocation site for ch2?")
		}

		anyFound := false
		for _, pset := range psets {
			if pset.Contains(ch2) {
				anyFound = true
				if pset.Size() != 1 {
					// There are no blocking operations on ch2, so no
					// dependency edges should be formed for it.
					t.Errorf("Expected pset containing ch2 to be a singleton set, was:\n%v", pset)
				}
			}
		}

		if !anyFound {
			t.Errorf("No pset contained %v (%v)", ch2, psets)
		}
	})
}
