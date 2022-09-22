package chreflect

import (
	"testing"

	"github.com/cs-au-dk/goat/testutil"

	"golang.org/x/tools/go/ssa"
)

func TestGetReflectedChannels(t *testing.T) {
	loadRes := testutil.LoadExamplePackage(t, "../../..", "reflect-ch")

	reflectedChans := GetReflectedChannels(loadRes.Prog, loadRes.Pointer)
	if reflectedChans.Size() != 1 {
		t.Fatal("Expected reflectedChans to contain 1 element")
	}

	fun := loadRes.Mains[0].Func("main")
	for _, insn := range fun.Blocks[0].Instrs {
		if mkChn, ok := insn.(*ssa.MakeChan); ok {
			if _, found := reflectedChans.Get(mkChn); !found {
				t.Error("Expected reflectedChans to contain", mkChn)
			}
		}
	}
}
