package absint

import (
	"fmt"

	T "github.com/cs-au-dk/goat/analysis/transition"
	"github.com/cs-au-dk/goat/utils"
)

// Successor pairs an abstract configuration denoting the successor
// with a labeled edge, where the label is denoted by the transition.
type Successor struct {
	configuration *AbsConfiguration
	transition    T.Transition
}

// Configuration extracts the abstract configuration from a successor relation.
func (succ Successor) Configuration() *AbsConfiguration {
	return succ.configuration
}

// Transition extracts the transition from a successor relation.
func (succ Successor) Transition() T.Transition {
	return succ.transition
}

func (succ Successor) PrettyPrint() {
	succ.transition.PrettyPrint()
	fmt.Println("Resulting superlocation:")
	succ.configuration.PrettyPrint()
}

func (succ Successor) String() string {
	return succ.transition.String() +
		"\nResulting superlocation:\n" +
		succ.configuration.String()
}

// Hash computes a 32-bit hash for a given successor by combining
// the hashes of the underlying configuration and the transition.
func (succ Successor) Hash() uint32 {
	return utils.HashCombine(
		succ.configuration.Hash(),
		succ.transition.Hash(),
	)
}

// DeriveConf derives another successor from the current successor, overridden
// with the given configuration. The derived successor inherits the transition
// label from the current successor.
func (succ Successor) DeriveConf(c *AbsConfiguration) Successor {
	return Successor{c, succ.transition}
}
