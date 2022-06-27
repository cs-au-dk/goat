package gotopo

import (
	"Goat/analysis/cfg"
	"Goat/pkgutil"
	"Goat/utils"
	"Goat/utils/graph"
	"Goat/utils/hmap"
	"Goat/utils/worklist"
	"go/token"
	T "go/types"
	"strings"

	uf "github.com/spakin/disjoint"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// PSet construction context
type psetCtxt struct {
	valid        utils.SSAValueSet
	chanChanDeps map[ssa.Value]utils.SSAValueSet
}

// GCatch style PSets
type PSets []utils.SSAValueSet

func blacklistPrimitive(p ssa.Value) bool {
	return pkgutil.CheckInGoroot(p.Parent())
	// if f := p.Parent(); f != nil {
	// 	pkg := strings.Split(f.Pkg.String(), " ")[1]
	// 	// fmt.Println(pkg, strings.Split(pkg, "/")[0] == "net")
	// 	// utils.Prompt()
	// 	if strings.Split(pkg, "/")[0] == "net" {
	// 		return true
	// 	}
	// }
	// return false
}

func getPrimitives(v ssa.Value, pt *pointer.Result) (res map[ssa.Value]struct{}) {
	res = make(map[ssa.Value]struct{})

	var rec func(v ssa.Value)
	rec = func(v ssa.Value) {
		for _, l := range pt.Queries[v].PointsTo().Labels() {
			p := l.Value()
			if p == nil || blacklistPrimitive(p) {
				return
			}

			// switch pi := p.(type) {
			// // case *ssa.MakeInterface:
			// // 	rec(pi.X)
			// // case *ssa.ChangeType:
			// // 	rec(pi.X)
			// default:
			if _, ok := p.Type().Underlying().(*T.Chan); ok {
				// ||
				// 	utils.IsNamedType(p.Type(), "sync", "Mutex") ||
				// 	utils.IsNamedType(p.Type(), "sync", "RWMutex") ||
				// 	utils.IsNamedType(p.Type(), "sync", "Cond") ||
				/* NOTE: Why was this restriction necessary?
				Most mutexes come from struct fields. */
				// true {
				res[p] = struct{}{}
			}
			// }
		}
	}

	rec(v)
	return
}

func (C psetCtxt) makeDependencyMapFromRootNode(
	CFG *cfg.Cfg,
	entry cfg.Node,
	D map[ssa.Value]map[ssa.Value]struct{},
	G graph.Graph[*ssa.Function],
	pt *pointer.Result) {
	visited := make(map[*ssa.Function]struct{})
	// Add v2 as a dependency of v1
	addDep := func(v1, v2 ssa.Value) {
		if _, ok := D[v1]; !ok {
			D[v1] = make(map[ssa.Value]struct{})
		}
		D[v1][v2] = struct{}{}
	}

	getPrimitives := func(v ssa.Value, pt *pointer.Result) map[ssa.Value]struct{} {
		res := getPrimitives(v, pt)
		for v := range res {
			if _, ok := C.valid.Get(v); !ok {
				delete(res, v)
			}
		}

		return res
	}

	worklist.Start(entry.Function(), func(f *ssa.Function, add func(el *ssa.Function)) {
		if _, ok := visited[f]; ok {
			return
		}

		in, out := CFG.FunIO(f)
		if in == nil || out == nil {
			return
		}
		// Remember what primitives may block the current instruction
		// via a preceding blocking operation
		BI := make(map[cfg.Node]utils.SSAValueSet)
		get := func(n cfg.Node) utils.SSAValueSet {
			if set, ok := BI[n]; !ok {
				return utils.MakeSSASet()
			} else {
				return set
			}
		}

		worklist.Start(in, func(n cfg.Node, add func(el cfg.Node)) {
			joinSucc := func(succ cfg.Node) {
				if succ == nil {
					return
				}

				if old, ok := BI[succ]; !ok {
					BI[succ] = get(n)
					add(succ)
				} else {
					new := old.Join(get(n))
					if old.Size() < new.Size() {
						BI[succ] = new
						add(succ)
					}
				}
			}

			addSuccs := func() {
				for succ := range n.Successors() {
					if n.Function() == succ.Function() {
						joinSucc(succ)
					}
				}
			}
			addBlocking := func(n cfg.Node, v ssa.Value) {
				for p := range getPrimitives(v, pt) {
					BI[n] = get(n).Add(p)
					add(n)
				}
			}
			addDependency := func(v ssa.Value) {
				for p := range getPrimitives(v, pt) {
					if _, ok := D[p]; !ok {
						D[p] = make(map[ssa.Value]struct{})
					}
					// Add every channel that had a blocking operation
					// preceding this instruction
					get(n).ForEach(func(v ssa.Value) {
						addDep(p, v)
					})
				}
			}

			// If x is a channel, and a payload of ch, create
			// a carrier dependency for x on ch
			addChanChanDependency := func(ch, x ssa.Value) {
				if _, ok := x.Type().Underlying().(*T.Chan); ok {
					for x := range getPrimitives(x, pt) {
						set := utils.MakeSSASet()
						if prev, ok := C.chanChanDeps[x]; ok {
							set = prev.Join(set)
						}
						for ch := range getPrimitives(ch, pt) {
							set = set.Add(ch)
						}

						C.chanChanDeps[x] = set
					}
				}
			}

			switch n := n.(type) {
			case *cfg.SSANode:
				switch i := n.Instruction().(type) {
				case *ssa.MakeChan:
					for p := range getPrimitives(i, pt) {
						if _, ok := D[p]; !ok {
							D[i] = make(map[ssa.Value]struct{})
						}
					}
				case *ssa.Call:
					// rcvr, cc := isConcurrentCall(i.Call)
					// switch cc {
					// case _BLOCKING_SYNC_CALL:
					// 	addBlocking(n.CallRelationNode(), rcvr)
					// 	fallthrough
					// case _SYNC_CALL:
					// 	addDependency(rcvr)
					// }
					joinSucc(n.CallRelationNode())
					return
				case *ssa.Panic:
					return
				case *ssa.Return:
					return
				case *ssa.UnOp:
					if i.Op == token.ARROW {
						addBlocking(n.Successor(), i.X)
						addDependency(i.X)
					}
				case *ssa.Send:
					addBlocking(n.Successor(), i.Chan)
					addDependency(i.Chan)
					addChanChanDependency(i.Chan, i.X)
				}
			case *cfg.DeferCall:
				// rcvr, cc := isConcurrentCall(n.Instruction().(*ssa.Defer).Call)
				// switch cc {
				// case _BLOCKING_SYNC_CALL:
				// 	addBlocking(n.CallRelationNode(), rcvr)
				// 	fallthrough
				// case _SYNC_CALL:
				// 	addDependency(rcvr)
				// }
				joinSucc(n.CallRelationNode())
				return
			case *cfg.BuiltinCall:
				if n.IsCommunicationNode() && n.Channel() != nil {
					addDependency(n.Channel())
				}
			case *cfg.SelectRcv:
				addBlocking(n.Successor(), n.Channel())
				addDependency(n.Channel())
			case *cfg.SelectSend:
				addBlocking(n.Successor(), n.Channel())
				addDependency(n.Channel())
				addChanChanDependency(n.Channel(), n.Val)
			case *cfg.Select:
				prims := utils.MakeSSASet()
				for _, s := range n.Insn.States {
					for p := range getPrimitives(s.Chan, pt) {
						prims = prims.Add(p)
					}
				}

				prims.ForEach(func(p1 ssa.Value) {
					prims.ForEach(func(p2 ssa.Value) {
						if p1 == p2 {
							return
						}
						addDep(p1, p2)
						addDep(p2, p1)
					})
				})
			}
			addSuccs()
		})

		if out.Function().Name() == "init" {
			for caller := range out.Successors() {
				if caller.Function().Pkg == out.Function().Pkg {
					add(caller.Function())
				}
			}
		}

		for _, next := range G.Edges(f) {
			add(next)
		}
		visited[f] = struct{}{}
	})
}

type psetConfig struct {
	CFG   *cfg.Cfg
	G     graph.Graph[*ssa.Function]
	entry cfg.Node
	chansInScope *utils.SSAValueSet
}

func makePSetCtxt(C psetConfig) (ctxt psetCtxt) {
	set := utils.MakeSSASet()

	visited := make(map[cfg.Node]struct{})

	worklist.Start(C.entry, func(n cfg.Node, add func(cfg.Node)) {
		if _, ok := visited[n]; ok {
			return
		}

		visited[n] = struct{}{}

		addCallees := func() {
			if n.Function() == nil {
				return
			}
			edges := C.G.Edges(n.Function())

			for succ := range n.Successors() {
				f := succ.Function()
				for _, edge := range edges {
					if edge == f {
						add(succ)
					}
				}
			}
			for succ := range n.Spawns() {
				f := succ.Function()
				for _, edge := range edges {
					if edge == f {
						add(succ)
					}
				}
			}


			if post := n.CallRelationNode(); post != nil {
				add(post)
			}
		}

		switch n := n.(type) {
		case *cfg.FunctionExit:
			return
		case *cfg.SSANode:
			switch i := n.Instruction().(type) {
			case *ssa.MakeChan:
				set = set.Add(i)
			case *ssa.Call:
				addCallees()
				return
			case *ssa.Go:
				addCallees()
			}
		case *cfg.DeferCall:
			addCallees()
			return
		}

		for succ := range n.Successors() {
			add(succ)
		}
	})

	ctxt.chanChanDeps = make(map[ssa.Value]utils.SSAValueSet)
	ctxt.valid = set
	if C.chansInScope != nil {
		ctxt.valid = ctxt.valid.Meet(*C.chansInScope)
	}

	return
}

// Get whole program GCatch style P-sets
func GetInterprocPsets(CFG *cfg.Cfg, pt *pointer.Result, G graph.Graph[*ssa.Function]) PSets {
	// Intra-procedural dependency map of channels
	D := make(map[ssa.Value]map[ssa.Value]struct{})

	for _, entry := range CFG.GetEntries() {
		if entry != nil && entry.Function() != nil {
			makePSetCtxt(psetConfig{CFG, G, entry, nil}).makeDependencyMapFromRootNode(CFG, entry, D, G, pt)
		}
	}

	return psetsFromDMap(D)
}

// Compute a single, whole program P-set that includes all channels.
func GetTotalPset(ps Primitives) (psets PSets) {
	set := utils.MakeSSASet()

	for _, usage := range ps {
		for ch := range usage.Chans() {
			set = set.Add(ch)
		}
	}

	psets = append(psets, set)

	return
}

func smallerScope(p1, p2 ssa.Value,
	computeDominator func(...*ssa.Function) *ssa.Function,
	CallDAG graph.SCCDecomposition[*ssa.Function],
	ps Primitives) bool {

	p1Funs := []*ssa.Function{}
	p2Funs := []*ssa.Function{}

	for fun, usageInfo := range ps {
		if CallDAG.ComponentOf(fun) == -1 {
			continue
		}

		if usageInfo.HasChan(p1) {
			p1Funs = append(p1Funs, fun)
		}
		if usageInfo.HasChan(p2) {
			p2Funs = append(p2Funs, fun)
		}
	}

	if len(p1Funs) == 0 || len(p2Funs) == 0 {
		return false
	}

	p1Dom := computeDominator(p1Funs...)
	p2Dom := computeDominator(p2Funs...)

	// Check if the dominator is in a "smaller" SCC
	return CallDAG.ComponentOf(p2Dom) <= CallDAG.ComponentOf(p1Dom)
}

func GetGCatchPSets(CFG *cfg.Cfg, f *ssa.Function, pt *pointer.Result,
	G graph.Graph[*ssa.Function],
	computeDominator func(...*ssa.Function) *ssa.Function,
	CallDAG graph.SCCDecomposition[*ssa.Function],
	ps Primitives,
) (psets PSets) {
	D := make(map[ssa.Value]map[ssa.Value]struct{})

	entry, _ := CFG.FunIO(f)

	chansInScope := new(utils.SSAValueSet)
	*chansInScope = ps.Chans()

	C := makePSetCtxt(psetConfig{CFG, G, entry, chansInScope})
	C.makeDependencyMapFromRootNode(CFG, entry, D, G, pt)

	PMap := make(map[ssa.Value]*uf.Element)

	// Pre-fill Psets for new primitives
	for p := range D {
		el := uf.NewElement()
		el.Data = p
		PMap[p] = el
	}

	for p1, ps := range D {
		for p2 := range ps {
			ps2 := D[p2]
			if _, ok := ps2[p1]; ok {
				uf.Union(PMap[p1], PMap[p2])
			}
		}
	}

	sets := make(map[*uf.Element]utils.SSAValueSet)

	for v, rep := range PMap {
		set, ok := sets[rep.Find()]

		if !ok {
			set = utils.MakeSSASet()
		}

		sets[rep.Find()] = set.Add(v)
	}

	seen := hmap.NewMap[bool](utils.SSAValueSetHasher)
	for _, set := range sets {
		if set.Empty() {
			continue
		}

		set.ForEach(func(p1 ssa.Value) {
			pset := utils.MakeSSASet(p1)

			set.ForEach(func(p2 ssa.Value) {
				if p1 != p2 && smallerScope(p1, p2, computeDominator, CallDAG, ps) {
					pset = pset.Add(p2)
				}
			})

			if carriers, ok := C.chanChanDeps[p1]; ok {
				pset = pset.Join(carriers)
			}

			if !seen.Get(pset) {
				seen.Set(pset, true)
				psets = append(psets, pset)
			}
		})
	}

	return psets
}

func psetsFromDMap(D map[ssa.Value]map[ssa.Value]struct{}) (psets PSets) {
	psets = make(PSets, 0)

	PMap := make(map[ssa.Value]*uf.Element)

	// Pre-fill Psets for new primitives
	for p := range D {
		el := uf.NewElement()
		el.Data = p
		PMap[p] = el
	}

	for p1, ps := range D {
		for p2 := range ps {
			ps2 := D[p2]
			if _, ok := ps2[p1]; ok {
				uf.Union(PMap[p1], PMap[p2])
			}
		}
	}

	sets := make(map[*uf.Element]utils.SSAValueSet)

	for v, rep := range PMap {
		set, ok := sets[rep.Find()]
		if !ok {
			set = utils.MakeSSASet()
		}

		sets[rep.Find()] = set.Add(v)
	}

	for _, set := range sets {
		if set.Size() > 0 {
			psets = append(psets, set)
		}
	}

	return
}

func (psets PSets) Get(v ssa.Value) utils.SSAValueSet {
	for _, pset := range psets {
		if _, ok := pset.Get(v); ok {
			return pset
		}
	}
	return utils.MakeSSASet()
}

func (psets PSets) String() string {
	strs := make([]string, 0, len(psets))

	for _, pset := range psets {
		strs = append(strs, pset.String())
	}

	return "Found Psets:\n" + strings.Join(strs, "\n")
}

func GetSingletonPsets(ps Primitives) (psets PSets) {
	seen := map[ssa.Value]bool{}
	for _, usage := range ps {
		for ch := range usage.Chans() {
			if !seen[ch] {
				seen[ch] = true
				psets = append(psets, utils.MakeSSASet(ch))
			}
		}
	}

	return
}

func GetSameFuncPsets(ps Primitives) (psets PSets) {
	// Make psets for primitives that are allocated in or used in the same function.
	// Primitives allocated in the same function are implicitly in the same pset from the beginning.
	// If two primitives allocated in different functions are used in the same
	// function, we join the psets of the two functions.
	elements := map[*ssa.Function]*uf.Element{}

	seenPrimitives := map[ssa.Value]struct{}{}

	for _, usageInfo := range ps {
		var repElement *uf.Element

		for _, usedPrimitives := range []map[ssa.Value]struct{}{
			usageInfo.Chans(),
			usageInfo.Sync(),
		} {
			for prim := range usedPrimitives {
				seenPrimitives[prim] = struct{}{}

				parent := prim.Parent()
				element := elements[parent]
				if element == nil {
					element = uf.NewElement()
					elements[parent] = element
				}

				if repElement == nil {
					repElement = element
				} else {
					uf.Union(repElement, element)
				}
			}
		}
	}

	psetMap := map[*uf.Element]utils.SSAValueSet{}
	for prim := range seenPrimitives {
		rep := elements[prim.Parent()].Find()
		set, found := psetMap[rep]
		if !found {
			set = utils.MakeSSASet()
		}

		psetMap[rep] = set.Add(prim)
	}

	for _, pset := range psetMap {
		psets = append(psets, pset)
	}

	return
}
