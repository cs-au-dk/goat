package transition

import (
	"Goat/analysis/defs"
	loc "Goat/analysis/location"
	"Goat/utils"
	"fmt"
	"sort"
	"strings"
)

type Wait struct {
	Progressed defs.Goro
	Cond       loc.Location
}

func (t Wait) PrettyPrint() {
	fmt.Println(t.Progressed, "started waiting on Cond", t.Cond)
}

func (t Wait) String() string {
	return t.Progressed.String() + "-[ Wait(" + t.Cond.String() + ") ]"
}

func (t Wait) Hash() uint32 {
	return utils.HashCombine(
		t.Progressed.Hash(),
		t.Cond.Hash(),
	)
}

type Wake struct {
	Progressed defs.Goro
	Cond       loc.Location
}

func (t Wake) PrettyPrint() {
	fmt.Println(t.Progressed, "started waking on Cond", t.Cond)
}

func (t Wake) String() string {
	return t.Progressed.String() + "-[ Wake(" + t.Cond.String() + ") ]"
}

func (t Wake) Hash() uint32 {
	return utils.HashCombine(
		t.Progressed.Hash(),
		t.Cond.Hash(),
	)
}

type Signal struct {
	Progressed1 defs.Goro
	Progressed2 defs.Goro
	Cond        loc.Location
}

func (t Signal) Missed() bool {
	return t.Progressed2 == nil
}

func (t Signal) PrettyPrint() {
	if t.Missed() {
		fmt.Println(t.Progressed1, "sends a missed signal via Cond", t.Cond)
	} else {
		fmt.Println(t.Progressed1, "signals", t.Progressed2, "to wake via Cond", t.Cond)
	}
}

func (t Signal) String() string {
	if t.Missed() {
		return t.Progressed1.String() + "-[ " + t.Cond.String() + ".Signal() ]-??"
	}
	return t.Progressed1.String() + "-[ " + t.Cond.String() + ".Signal() ]-" + t.Progressed2.String()
}

func (t Signal) Hash() uint32 {
	var t2Hash uint32
	if !t.Missed() {
		t2Hash = t.Progressed2.Hash()
	}
	return utils.HashCombine(
		t.Progressed1.Hash(),
		t2Hash,
		t.Cond.Hash(),
	)
}

type Broadcast struct {
	Broadcaster  defs.Goro
	Broadcastees map[defs.Goro]struct{}
	Cond         loc.Location
}

func (t Broadcast) PrettyPrint() {
	fmt.Println(t.Broadcaster, "broadcasts via Cond", t.Cond, "to the following goroutines:")
	for g := range t.Broadcastees {
		fmt.Println("--", g)
	}
}

func (t Broadcast) String() string {
	str := t.Broadcaster.String() + "-[ " + t.Cond.String() + ".Broadcast() ]-[ "

	gs := make([]defs.Goro, 0, len(t.Broadcastees))
	for g := range t.Broadcastees {
		gs = append(gs, g)
	}

	sort.Slice(gs, func(i, j int) bool {
		return gs[i].Hash() < gs[j].Hash()
	})

	gstrs := make([]string, 0, len(gs))
	for _, g := range gs {
		gstrs = append(gstrs, g.String())
	}

	return str + strings.Join(gstrs, "; ") + " ]"
}

func (t Broadcast) Hash() uint32 {
	bs := make([]uint32, 0, len(t.Broadcastees))
	for g := range t.Broadcastees {
		bs = append(bs, g.Hash())
	}
	sort.SliceStable(bs, func(i, j int) bool {
		return bs[i] < bs[j]
	})

	return utils.HashCombine(
		t.Broadcaster.Hash(),
		t.Cond.Hash(),
		utils.HashCombine(bs...))
}
