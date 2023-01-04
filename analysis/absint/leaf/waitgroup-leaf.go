package leaf

import (
	"github.com/cs-au-dk/goat/analysis/cfg"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
)

type (
	// WaitGroupAdd is a communication leaf where a thread increments
	// the specified WaitGroup with a Delta.
	WaitGroupAdd struct {
		cfg.Synthetic
		Loc   loc.Location
		Delta L.AbstractValue
	}

	// WaitGroupWait is a communication leaf where the WaitGroup allocated at Loc
	// will wait for the counter to become 0. The CFN set as the predecessor
	// to a WaitGroup leaf represents the source location.
	WaitGroupWait struct {
		cfg.Synthetic
		Loc loc.Location
	}
)
