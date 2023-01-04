package transition

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/defs"
)

type (
	// Transition is implemented by all superlocation graph edges.
	Transition interface {
		Hash() uint32
		String() string
		PrettyPrint()
	}

	// TransitionSingle is implemented by all single-thread transitions.
	TransitionSingle interface {
		Progressed() defs.Goro
	}

	// transitionSingle is a transition that progresses a single goroutine
	transitionSingle struct{ progressed defs.Goro }

	// In is an internal (non-communicating) transition
	In struct{ transitionSingle }
)

// Hash computes a 32-bit hash for the given transition.
func (t transitionSingle) Hash() uint32 { return t.progressed.Hash() }

// Progressed returns the thread progressed by the transition.
func (t transitionSingle) Progressed() defs.Goro { return t.progressed }

func (t In) String() string {
	return "𝜏" + t.progressed.String()
}

// PrettyPrint prints the internal transition.
func (t In) PrettyPrint() {
	fmt.Println("Internal transition for thread", t.progressed)
}

// NewIn creates a new internal transition.
func NewIn(progressed defs.Goro) In {
	return In{transitionSingle{progressed}}
}
