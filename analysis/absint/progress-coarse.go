package absint

import (
	"fmt"
	"log"

	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
)

// Takes an abstract configuration and tries to progress threads as far as possible using only silent transitions.
// If a goroutine can end up in multiple places it is not progressed further.
func CoarseProgress(C AnalysisCtxt) (
	*AbsConfiguration,
	L.AnalysisState,
) {
	conf := C.InitConf
	state := C.InitState
	done := make(map[uint32]bool)
	newConf := conf.Copy()

	for {
		// Find a thread to progress that is not at a communication node and isn't done
		tid, cl, found := newConf.Superloc.Find(func(tid defs.Goro, cl defs.CtrLoc) bool {
			return !cl.Node().IsCommunicationNode() && !done[tid.Hash()] && !cl.Panicked()
		})

		if !found {
			break
		} else if opts.Verbose() {
			log.Printf("Choosing goroutine %s at control location %s", tid, cl)
		}

		// Keep progressing until hitting a communication node, done, or panic
		for !newConf.GetUnsafe(tid).Node().IsCommunicationNode() &&
			!done[tid.Hash()] && !newConf.IsPanicked() {

			succs := newConf.GetSilentSuccessors(C, tid, state)
			if len(succs) == 0 {
				panic(fmt.Errorf("0 silent successors from %s: %s", tid, newConf.Superloc.GetUnsafe(tid)))
			} else if len(succs) != 1 {
				// Multiple successors - mark as done.
				done[tid.Hash()] = true
			} else {
				for _, succ := range succs {
					newConf = succ.Configuration()
					state = succ.State
				}
			}
		}
	}

	opts.OnVerbose(func() {
		log.Println("Progressed conf:")
		newConf.PrettyPrint()
	})

	return newConf, state
}
