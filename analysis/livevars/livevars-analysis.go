package livevars

import (
	"log"

	"github.com/cs-au-dk/goat/analysis/cfg"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/worklist"

	"github.com/benbjohnson/immutable"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

func findExits(n cfg.Node) (exits map[cfg.Node]bool) {
	exits = make(map[cfg.Node]bool)
	// Relies on CFG compression to succeed
	if n.DeferLink() != nil {
		exits[n.DeferLink()] = true
		return
	}

	visited := make(map[cfg.Node]bool)
	var visit func(cfg.Node)
	visit = func(n cfg.Node) {
		if _, ok := visited[n]; !ok {
			visited[n] = true
			switch n := n.(type) {
			case *cfg.SSANode:
				switch n.Instruction().(type) {
				case *ssa.Call:
					visit(n.CallRelationNode())
					return
				}
			}
			for succ := range n.Successors() {
				visit(succ)
			}
		} else {
			exits[n] = true
		}
	}
	visit(n)

	return
}

func LiveVars(G cfg.Cfg, pt *pointer.Result) *immutable.Map {
	log.Println("Starting channel liveness analysis...")

	getAllMakeChans(G)

	chanLattice := L.Create().Lattice().Powerset(chans)
	setFactory := L.Create().Element().Powerset(chanLattice)

	/* TODO: Temporarily replaced with a simple map to specify a custom hasher
	liveLattice := L.Create().Lattice().Map(L.Lift(chanLattice), nodes)
	liveVars := L.Create().Element().Map(liveLattice)(nil)
	*/
	liftedChanLattice := L.Lift(chanLattice)
	liveVars := immutable.NewMap(utils.PointerHasher{})
	getOrBot := func(key cfg.Node) L.Element {
		if val, ok := liveVars.Get(key); ok {
			return val.(L.Element)
		} else {
			return liftedChanLattice.Bot()
		}
	}

	W := worklist.Empty[cfg.Node]()

	Transfer := func(n cfg.Node) (result L.Element) {

		// Retrieve node lattice element
		nodeVar := getOrBot(n)
		switch nodeVar := nodeVar.(type) {
		case *L.LiftedBot:
			result = setFactory(nil)
		case L.Set:
			result = setFactory(nodeVar.All())
		}

		for succ := range n.Successors() {
			succVar := getOrBot(succ)
			result = result.Join(succVar)
		}

		for spawn := range n.Spawns() {
			spawnVar := getOrBot(spawn)
			result = result.Join(spawnVar)
		}
		set := result.Set()

		switch n := n.(type) {
		case *cfg.SSANode:
			switch i := n.Instruction().(type) {
			case *ssa.MakeChan:
				set = set.Remove(i)
				return set
			}
			for _, v := range n.Instruction().Operands([]*ssa.Value{}) {
				p := pt.Queries[*v].PointsTo()
				for _, l1 := range p.Labels() {
					switch chn := l1.Value().(type) {
					case *ssa.MakeChan:
						set = set.Add(chn)
					}
				}
			}
		case *cfg.SelectSend:
			p := pt.Queries[n.Channel()].PointsTo()
			for _, l1 := range p.Labels() {
				switch chn := l1.Value().(type) {
				case *ssa.MakeChan:
					set = set.Add(chn)
				}
			}
		case *cfg.SelectRcv:
			p := pt.Queries[n.Channel()].PointsTo()
			for _, l1 := range p.Labels() {
				switch chn := l1.Value().(type) {
				case *ssa.MakeChan:
					set = set.Add(chn)
				}
			}
		}

		return set
	}

	for _, entry := range G.GetEntries() {
		if entry.Function().Name() == "init" {
			for exit := range findExits(entry) {
				switch exit.(type) {
				case *cfg.FunctionExit:
					for exit := range findExits(exit.Successor()) {
						W.Add(exit)
					}
				default:
					W.Add(exit)
				}
			}
		} else {
			for exit := range findExits(entry) {
				W.Add(exit)
			}
		}
	}

	for !W.IsEmpty() {
		v := W.GetNext()
		vVar := getOrBot(v)
		up := Transfer(v)
		if !vVar.Eq(up) {
			liveVars = liveVars.Set(v, up)
			for w := range v.Predecessors() {
				W.Add(w)
			}
		}
		for spawn := range v.Spawns() {
			for exit := range findExits(spawn) {
				W.Add(exit)
			}
		}
	}
	log.Println("Variable liveness finished")
	return liveVars
}
