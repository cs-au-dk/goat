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

type In struct {
	Progressed defs.Goro
}

func (t In) Hash() uint32 {
	return t.Progressed.Hash()
}

func (t In) String() string {
	return "𝜏" + t.Progressed.String()
}

func (t In) PrettyPrint() {
	fmt.Println("Internal transition for thread", t.Progressed)
}
