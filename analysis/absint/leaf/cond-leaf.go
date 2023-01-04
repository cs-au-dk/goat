package leaf

import (
	"github.com/cs-au-dk/goat/analysis/cfg"
	loc "github.com/cs-au-dk/goat/analysis/location"

	"golang.org/x/tools/go/ssa"
)

type (
	// condLeafSynthetic is a base-line struct embedded by all
	// Cond communication leaves.
	condLeafSynthetic struct {
		cfg.Synthetic
		Loc loc.Location
	}
	// CondWaiting is a communication leaf where the Cond allocated at Loc
	// is waiting to be signalled by another thread. The CF-node set as the
	// predecessor to a CondWaiting leaf represents the source location.
	CondWaiting struct {
		condLeafSynthetic
	}
	// CondWaking is a communication leaf where the Cond allocated at Loc
	// is awakening after being signalled by another thread. The CF-node set
	// as the predecessor to a CondWaking leaf represents the source location.
	CondWaking struct {
		condLeafSynthetic
	}
	// CondWait is a communication leaf where the Cond allocated at Loc
	// will attempt to start waiting. The CF-node set as the predecessor
	// to a CondWait leaf represents the source location.
	CondWait struct {
		condLeafSynthetic
	}
	// CondSignal is a communication leaf where the Cond allocated at Loc
	// may signal to another thread waiting at Loc to wake. The CF-node set as
	// the predecessor a CondSignal leaf represents the source location.
	CondSignal struct {
		condLeafSynthetic
	}
	// CondBroadcast is a communication leaf where the Cond allocated at Loc
	// may signal to all other threads waiting at Loc to wake. The CF-node set as
	// the predecessor a CondBroadcast leaf represents the source location.
	CondBroadcast struct {
		condLeafSynthetic
	}
)

// Cond returns the SSA value of the Cond operand
// in the underlying CF-node Cond operation.
func (n *condLeafSynthetic) Cond() ssa.Value {
	return n.Predecessor().Cond()
}
