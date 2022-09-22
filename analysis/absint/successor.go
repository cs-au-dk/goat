package absint

import (
	"fmt"

	T "github.com/cs-au-dk/goat/analysis/transition"
	"github.com/cs-au-dk/goat/utils"
)

// Basic successor implementation for any configuration. Successors at
// different abstraction levels should embed it. Includes
// a description of the transition, and the succeeding configuration.
type Successor struct {
	configuration *AbsConfiguration
	transition    T.Transition
}

func (succ Successor) Configuration() *AbsConfiguration {
	return succ.configuration
}

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

func (succ Successor) Hash() uint32 {
	return utils.HashCombine(
		succ.configuration.Hash(),
		succ.transition.Hash(),
	)
}

func (succ Successor) DeriveConf(c *AbsConfiguration) Successor {
	return Successor{c, succ.transition}
}
