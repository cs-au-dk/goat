package transition

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/defs"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

type Sync struct {
	Channel     loc.Location
	Progressed1 defs.Goro
	Progressed2 defs.Goro
}

func (t Sync) Hash() uint32 {
	var t1, t2 uint32
	if h1, h2 := t.Progressed1.Hash(), t.Progressed2.Hash(); h1 < h2 {
		t1, t2 = h1, h2
	} else {
		t2, t1 = h1, h2
	}
	return utils.HashCombine(
		t1,
		t2,
		t.Channel.Hash(),
	)
}

func (t Sync) PrettyPrint() {
	/* TODO: Maybe try to use channel names when we can map from concrete location to allocation site
	if name, ok := upfront.ChannelNames[t.Channel.Pos()]; ok {
		fmt.Println("Synchronized threads", t.Progressed1, "-", t.Sync1, "and", t.Progressed2, "-", t.Sync2, "on channel:", name, utils.SSAValString(t.Channel))
		return
	}
	*/
	fmt.Println("Synchronized threads", t.Progressed1, "and", t.Progressed2, "on channel:", t.Channel)
}

func (t Sync) String() (str string) {
	str = ""
	h1 := t.Progressed1.Hash()
	h2 := t.Progressed2.Hash()
	if h1 < h2 {
		str += t.Progressed1.String() + "-" + t.Progressed2.String()
	} else {
		str += t.Progressed2.String() + "-" + t.Progressed1.String()
	}
	return str + "<" + t.Channel.String() + ">"
}

type Close struct {
	transitionSingle
	Op ssa.Value
}

func (t Close) PrettyPrint() {
	fmt.Println("Performed close operation", t.Op, "on thread", t.progressed)
}

func (t Close) String() (str string) {
	return t.progressed.String() + ": ■" + utils.SSAValString(t.Op) + ""
}

func (t Close) Hash() uint32 {
	phasher := utils.PointerHasher[ssa.Value]{}
	return utils.HashCombine(t.progressed.Hash(), phasher.Hash(t.Op))
}

func NewClose(progressed defs.Goro, chn ssa.Value) Close {
	return Close{transitionSingle{progressed}, chn}
}

type Receive struct {
	transitionSingle
	Chan loc.Location
}

func (t Receive) PrettyPrint() {
	fmt.Println("Performed buffered receive operation on", t.Chan, "on thread", t.progressed)
}

func (t Receive) String() (str string) {
	return t.progressed.String() + ": ←" + t.Chan.String()
}

func (t Receive) Hash() uint32 {
	return utils.HashCombine(t.progressed.Hash(), t.Chan.Hash())
}

func NewReceive(progressed defs.Goro, chn loc.Location) Receive {
	return Receive{transitionSingle{progressed}, chn}
}

type Send struct {
	transitionSingle
	Chan loc.Location
}

func (t Send) PrettyPrint() {
	fmt.Println("Performed buffered send operation on", t.Chan, "on thread", t.progressed)
}

func (t Send) String() (str string) {
	return t.progressed.String() + ": " + t.Chan.String() + "←"
}

func (t Send) Hash() uint32 {
	return utils.HashCombine(t.progressed.Hash(), t.Chan.Hash())
}

func NewSend(progressed defs.Goro, chn loc.Location) Send {
	return Send{transitionSingle{progressed}, chn}
}
