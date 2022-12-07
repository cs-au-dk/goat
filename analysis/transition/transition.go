package transition

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/defs"
)

type Transition interface {
	Hash() uint32
	String() string
	PrettyPrint()
}

// Sub-interface for single-goro transitions
type TransitionSingle interface {
	Progressed() defs.Goro
}

type transitionSingle struct{ progressed defs.Goro }

func (t transitionSingle) Hash() uint32          { return t.progressed.Hash() }
func (t transitionSingle) Progressed() defs.Goro { return t.progressed }

type In struct{ transitionSingle }

func (t In) String() string {
	return "𝜏" + t.progressed.String()
}

func (t In) PrettyPrint() {
	fmt.Println("Internal transition for thread", t.progressed)
}

func NewIn(progressed defs.Goro) In {
	return In{transitionSingle{progressed}}
}
