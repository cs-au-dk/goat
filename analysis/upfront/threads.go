package upfront

import (
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// collectSpawns gathers all `go` instructions in an SSA program.
func collectSpawns(prog *ssa.Program) (threads map[*ssa.Go]bool) {
	threads = make(map[*ssa.Go]bool)

	for fun := range ssautil.AllFunctions(prog) {
		for _, b := range fun.Blocks {
			for _, i := range b.Instrs {
				switch thread := i.(type) {
				case *ssa.Go:
					threads[thread] = true
				}
			}
		}
	}

	return
}
