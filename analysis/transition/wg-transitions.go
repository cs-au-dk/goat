package transition

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/utils"
)

type Add struct {
	transitionSingle
	WaitGroup loc.Location
	Amount    L.AbstractValue
}

func (t Add) PrettyPrint() {
	fmt.Printf("Adding %v to %v on thread %v", t.Amount, t.WaitGroup, t.progressed)
}

func (t Add) String() (str string) {
	return fmt.Sprintf("%v-[ Add(%v, %v) ]", t.progressed, t.WaitGroup, t.Amount)
}

func (t Add) Hash() uint32 {
	return utils.HashCombine(t.progressed.Hash(), t.WaitGroup.Hash())
}

func NewAdd(progressed defs.Goro, cond loc.Location, amount L.AbstractValue) Add {
	return Add{transitionSingle{progressed}, cond, amount}
}

type WaitGroupWait struct {
	transitionSingle
	WaitGroup loc.Location
}

func (t WaitGroupWait) PrettyPrint() {
	fmt.Println(t.progressed, "started waiting on WaitGroup", t.WaitGroup)
}

func (t WaitGroupWait) String() string {
	return t.progressed.String() + "-[ WGWait(" + t.WaitGroup.String() + ") ]"
}

func (t WaitGroupWait) Hash() uint32 {
	return utils.HashCombine(
		t.progressed.Hash(),
		t.WaitGroup.Hash(),
	)
}

func NewWaitGroupWait(progressed defs.Goro, cond loc.Location) WaitGroupWait {
	return WaitGroupWait{transitionSingle{progressed}, cond}
}
