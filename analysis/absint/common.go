package absint

import (
	"fmt"
	"log"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/pkgutil"
	tu "github.com/cs-au-dk/goat/testutil"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

var (
	opts = utils.Opts()
)

var (
	Lattices = L.Create().Lattice
	Elements = L.Create().Element
)

// Backwards compat.
func (C *AnalysisCtxt) setFragmentPredicate(Localized, AnalyzeCallsWithoutConcurrencyPrimitives bool) {
	C.FragmentPredicate = func(callIns ssa.CallInstruction, sfun *ssa.Function) bool {
		// Wrapper functions have no .Pkg field but are cheap to analyze.
		if sfun.Pkg == nil {
			return true
		}

		// Methods on sync.Once are easy to handle.
		if recv := sfun.Signature.Recv(); recv != nil &&
			utils.IsNamedType(recv.Type(), "sync", "Once") {
			return true
		}

		// Our heuristic in ValHasConcurrencyPrimitives is not strong enough to
		// see that contexts carry a "closed" channel (because it's hidden in
		// an atomic.Value).
		if sfun.Pkg.Pkg.Name() == "context" {
			return true
		}

		if !AnalyzeCallsWithoutConcurrencyPrimitives {
			// Determine whether a function involves concurrency primitives.
			// It does, if any of its parameters, free variables or its return type
			// carries a concurrency primitive
			for _, p := range sfun.Params {
				if utils.ValHasConcurrencyPrimitives(p, C.LoadRes.Pointer) {
					return true
				}
			}

			for _, p := range sfun.FreeVars {
				if utils.ValHasConcurrencyPrimitives(p, C.LoadRes.Pointer) {
					return true
				}
			}

			if v, ok := callIns.(ssa.Value); ok {
				return utils.ValHasConcurrencyPrimitives(v, C.LoadRes.Pointer)
			}
		}

		if !Localized {
			return pkgutil.IsLocal(sfun)
		}
		return false
	}
}

type prepAI struct {
	metrics bool
	log     bool
}

type AIConfig = struct {
	Metrics bool
	Log     bool
}

// Prepare Abstract Interpretation based on a
// configuration (e. g. collect metrics)
func ConfigAI(c AIConfig) prepAI {
	return prepAI{
		metrics: c.Metrics,
		log:     c.Log,
	}
}

// Interface to prepare abstract interpretation.
// Depending on preparation, generates a different analysis context
func PrepareAI() prepAI {
	return prepAI{}
}

// Prepare abstract interpretation for top level execution.
// Makes use of the executable flags and other options.
// Since a top-level execution may involve analysis of all functions,
// it produces a set of analysis contexts
func (p prepAI) Executable(loadRes tu.LoadResult) map[*ssa.Function]AnalysisCtxt {
	mainPkg := pkgutil.GetMain(loadRes.Mains)
	var root, entry *ssa.Function
	switch {
	case opts.IsWholeProgramAnalysis():
		ctxts := map[*ssa.Function]AnalysisCtxt{}
		if mainPkg != nil {
			root = mainPkg.Func("main")
			entry = mainPkg.Func("init")
			ctxts[entry] = p.prep(root, entry, loadRes, false)
		}

		// If the option to include test functions was selected,
		// also create analysis contexts for them.
		/* TODO: The analysis context is top injected for now,
		but the analysis will expand no-concurrency-impact calls,
		as if it were whole program analysis. This means that the
		testing variable is also top-injected, when it could be treated
		as a definitive singular object. */
		if opts.IncludeTests() {
			for _, tentry := range loadRes.Cfg.GetEntries() {
				f := tentry.Function()
				if f != entry {
					C := p.prep(f, f, loadRes, true)
					/*
						C.AnalyzeCallsWithoutConcurrencyPrimitives = true
						C.Localized = false
					*/
					C.setFragmentPredicate(false, true)
					ctxts[f] = C
				}
			}
		}

		return ctxts
	case !opts.AnalyzeAllFuncs():
		root = loadRes.Cfg.FunctionByName(opts.Function())
		entry = root

		return map[*ssa.Function]AnalysisCtxt{
			entry: p.prep(root, entry, loadRes, !opts.IsWholeProgramAnalysis()),
		}
	default:
		ctxts := make(map[*ssa.Function]AnalysisCtxt)
		for _, mem := range mainPkg.Members {
			if f, ok := mem.(*ssa.Function); ok {
				// Skip stand-alone analysis of functions
				// in internal libraries
				if pkgutil.CheckInGoroot(f) {
					continue
				}
				if f == mainPkg.Func("init") {
					continue
				}
				if f == mainPkg.Func("main") {
					root = mainPkg.Func("main")
					entry = mainPkg.Func("main")
					ctxts[entry] = p.prep(root, entry, loadRes, !opts.IsWholeProgramAnalysis())
				} else {
					if _, ok := loadRes.Cfg.Functions()[f]; ok {
						ctxts[f] = p.Function(f)(loadRes)
					}
				}
			}
		}

	NON_MAIN_PACKAGES:
		for _, pkg := range pkgutil.AllPackages(loadRes.Prog) {
			for _, mp := range loadRes.Mains {
				if pkg == mp {
					continue NON_MAIN_PACKAGES
				}
			}
			for _, mem := range pkg.Members {
				if f, ok := mem.(*ssa.Function); ok {
					// Skip stand-alone analysis of functions
					// in internal libraries
					if pkgutil.CheckInGoroot(f) {
						continue
					}
					if f == mainPkg.Func("init") {
						continue
					}
					if _, ok := loadRes.Cfg.Functions()[f]; ok {
						ctxts[f] = p.Function(f)(loadRes)
					}
				}
			}
		}

		return ctxts
	}
}

// Prepare abstract interpretation for whole program analysis
func (p prepAI) WholeProgram(loadRes tu.LoadResult) AnalysisCtxt {
	mainPkg := pkgutil.GetMain(loadRes.Mains)

	return p.prep(
		mainPkg.Func("main"),
		mainPkg.Func("init"),
		loadRes, false,
	)
}

type FragmentPredicate = func(ssa.CallInstruction, *ssa.Function) bool
type AnalysisCtxt struct {
	Function  *ssa.Function
	LoadRes   tu.LoadResult
	InitConf  *AbsConfiguration
	InitState L.AnalysisState

	// Determines which functions to "expand", essentially defining the
	// fragment of the program to analyze.
	FragmentPredicate FragmentPredicate

	// Akin to "PSet" in GCatch
	FocusedPrimitives []ssa.Value

	// Metrics collection
	Metrics *Metrics

	// Is AI logging enabled?
	// Current superloc
	Log struct {
		Enabled         bool
		MaxSuperloc     *int
		MaxPointsToSize *int
		MaxMemHeight    *int
		Superloc        defs.Superloc
		CtrLocVisits    map[defs.Superloc]map[defs.Goro]map[defs.CtrLoc]*struct {
			count int
			mem   L.Memory
			sl    defs.Superloc
		}
		mostVisitedCtrLoc   *defs.CtrLoc
		SuperlocationsFound map[defs.Superloc]struct{}
	}
}

func (C AnalysisCtxt) CheckMaxSuperloc(s defs.Superloc, spawnee defs.Goro) {
	if C.Log.Enabled && *C.Log.MaxSuperloc < s.Size()+1 {
		*C.Log.MaxSuperloc = s.Size() + 1
		log.Println("Latest superlocation size increase is:", *C.Log.MaxSuperloc)
		fmt.Println("At superlocation", s)
		var posStr string
		if n, ok := spawnee.CtrLoc().Node().(*cfg.SSANode); ok &&
			n.Instruction().Parent() != nil &&
			n.Instruction().Parent().Prog != nil {
			prog := n.Instruction().Parent().Prog
			posStr += prog.Fset.Position(n.Pos()).String()
		}
		fmt.Println("With spawnee", spawnee, "at", posStr)
	}
}

func (C AnalysisCtxt) CheckPointsTo(v L.PointsTo) {
	const LARGE_PT_SET = 20

	if C.Log.Enabled {
		size := v.Size()
		if *C.Log.MaxPointsToSize < size {
			log.Println("Largest points-to set size found so far has size", size)
			*C.Log.MaxPointsToSize = size
		}
		// if size > 20 {
		// 	log.Println("Large points-to set encountered", size)
		// }
	}
}

func (C AnalysisCtxt) CheckMemory(m L.Memory) {
	if C.Log.Enabled {
		// size := m.Height()
		// if *C.MaxMemHeight < size {
		// 	log.Println("Largest memory height found so far has size", size)
		// 	*C.MaxMemHeight = size
		// }
	}
}
func representative(l loc.AddressableLocation) (loc.AllocationSiteLocation, bool) {
	switch l := l.(type) {
	case loc.AllocationSiteLocation:
		s, _ := l.GetSite()
		return loc.AllocationSiteLocation{
			Site:    s,
			Goro:    defs.Create().TopGoro(),
			Context: s.Parent(),
		}, true
	default:
		return loc.AllocationSiteLocation{}, false
	}
}

func (C AnalysisCtxt) LogSuperlocation(sl defs.Superloc) {
	if C.Log.Enabled {
		_, seen := C.Log.SuperlocationsFound[sl]
		if seen {
			return
		}
		C.Log.SuperlocationsFound[sl] = struct{}{}
		if len(C.Log.SuperlocationsFound)%100 == 0 {
			fmt.Println("Found", len(C.Log.SuperlocationsFound), "superlocations globally")
		}
	}
}

func (C *AnalysisCtxt) LogWildcardSwap(m L.Memory, l loc.Location) {
	if C.Log.Enabled {
		site, found := l.GetSite()

		if !found || site.Parent() == nil || site.Parent().Prog == nil || !site.Pos().IsValid() {
			return
		}

		pos := site.Parent().Prog.Fset.Position(site.Pos()).String()
		log.Println("Swapped wildcard for ", l, "at", pos)
	}
}

func (C AnalysisCtxt) LogCtrLocMemory(g defs.Goro, cl defs.CtrLoc, m L.Memory) {
	if C.Log.Enabled && cl.Forking() {
		if _, seen := C.Log.CtrLocVisits[C.Log.Superloc]; !seen {
			C.Log.CtrLocVisits[C.Log.Superloc] = make(map[defs.Goro]map[defs.CtrLoc]*struct {
				count int
				mem   L.Memory
				sl    defs.Superloc
			})
		}
		if _, seen := C.Log.CtrLocVisits[C.Log.Superloc][g]; !seen {
			C.Log.CtrLocVisits[C.Log.Superloc][g] = make(map[defs.CtrLoc]*struct {
				count int
				mem   L.Memory
				sl    defs.Superloc
			})
		}

		ctrLocVisits := C.Log.CtrLocVisits[C.Log.Superloc][g]
		BOT := L.Create().Element().Memory()

		if _, ok := ctrLocVisits[cl]; !ok {
			ctrLocVisits[cl] = new(struct {
				count int
				mem   L.Memory
				sl    defs.Superloc
			})
			ctrLocVisits[cl].mem = BOT
			ctrLocVisits[cl].sl = C.Log.Superloc
		}
		ctrLocVisits[cl].count++
		oldMem := ctrLocVisits[cl].mem
		if ctrLocVisits[cl].count%10 == 0 {
			diff := oldMem.Difference(m)
			if !diff.Eq(BOT) {
				log.Println("Frequently revisited control location", cl, "in superlocation", C.Log.Superloc)
				fmt.Println("Visited", ctrLocVisits[cl].count, "times")
				fmt.Println("Allocated memory size before:", oldMem.EffectiveSize(),
					"after:", m.EffectiveSize())
				// if oldMem.EffectiveSize() > m.EffectiveSize() {
				oldMem.ForEach(func(al loc.AddressableLocation, av L.AbstractValue) {
					if _, found := m.Get(al); !found && !av.IsBot() {
						fmt.Println("Location in old memory not found in new memory:", al)
						fmt.Println("Value", av)
						Tl, ok := representative(al)
						if ok {
							fmt.Println("Top location:", Tl)
							_, found := m.Get(Tl)
							fmt.Println("Top location found?", found)
							// utils.Prompt()
						}
					}
				})
				fmt.Println("Memory increments for existing locations:")
				diff.ForEach(func(al loc.AddressableLocation, av L.AbstractValue) {
					v1, _ := oldMem.Get(al)
					v2, _ := m.Get(al)
					fmt.Println("Location:", al)
					fmt.Println("Before", v1)

					fmt.Println("After", v2)
					fmt.Println("Difference", av)
					// utils.Prompt()
				})
				// }

				// before, after := oldMem.Difference(m)
				// before.ForEach(func(al loc.AddressableLocation, av L.AbstractValue) {
				// 	if av.Eq(L.Consts().BotValue()) {
				// 		return
				// 	}
				// 	fmt.Println(al)
				// 	fmt.Println("Before:", av)
				// 	fmt.Println("After:", after.GetUnsafe(al))
				// })
				// utils.Prompt()
			}
			ctrLocVisits[cl].mem = m
		}
		// if C.CtrLocVisits[*C.mostVisitedCtrLoc] < C.CtrLocVisits[cl] {
		// 	*C.mostVisitedCtrLoc = cl
		// 	fmt.Println("Found new most visited control location", cl, "visted", C.CtrLocVisits[cl], "times")
		// }
	}
}

// Prepare provided SSA functions for abstract interpretation.
// Requires a root and entry function, a load result, and an indicator
// as to whether the abstract interpretation will be performed on a
// harnessed function. For harnessed functions, parameters and free variables
// will be instantiated to top.
func (p prepAI) prep(
	root *ssa.Function,
	entryFun *ssa.Function,
	loadRes tu.LoadResult,
	isHarnessed bool) AnalysisCtxt {

	// Define analysis entry node
	entry, _ := loadRes.Cfg.FunIO(entryFun)
	if entry == nil {
		panic(fmt.Errorf("%v does not have an entry in the CFG", entryFun))
	}

	// Define analysis entry control location
	cl := defs.Create().CtrLoc(entry, root, false)
	// Define entry goroutine
	goro := defs.Create().Goro(cl, nil)
	// Define entry superlocation (configuration)
	s0 := Create().AbsConfiguration(ABS_COARSE).DeriveThread(goro, cl)
	s0.Target = goro

	// Define initial state
	initState := Elements().AnalysisState(
		L.PopulateGlobals(
			Lattices().Memory().Bot().Memory(),
			loadRes.Prog.AllPackages(),
			isHarnessed,
		),
		Elements().ThreadCharges(),
	)

	if isHarnessed {
		vals := make([]loc.AddressableLocation, 0, len(entryFun.Params)+len(entryFun.FreeVars))

		for _, p := range entryFun.Params {
			vals = append(vals, loc.LocationFromSSAValue(goro, p))
		}

		for _, fv := range entryFun.FreeVars {
			vals = append(vals, loc.LocationFromSSAValue(goro, fv))
		}
		mem := initState.Memory()
		initState = initState.UpdateMemory(
			mem.InjectTopValues(vals...))
	}

	C := AnalysisCtxt{
		Function:  entryFun,
		LoadRes:   loadRes,
		InitConf:  s0,
		InitState: initState,
		Metrics:   p.InitializeMetrics()(entryFun),
	}

	if p.log {
		C.Log.Enabled = true
		C.Log.MaxSuperloc = new(int)
		C.Log.MaxPointsToSize = new(int)
		C.Log.MaxMemHeight = new(int)
		C.Log.CtrLocVisits = make(map[defs.Superloc]map[defs.Goro]map[defs.CtrLoc]*struct {
			count int
			mem   L.Memory
			sl    defs.Superloc
		})
		C.Log.SuperlocationsFound = make(map[defs.Superloc]struct{})
		C.Log.mostVisitedCtrLoc = new(defs.CtrLoc)
	}
	/*
		Localized:                                isHarnessed,
		AnalyzeCallsWithoutConcurrencyPrimitives: !isHarnessed,
	*/
	C.setFragmentPredicate(isHarnessed, !isHarnessed)
	return C
}

// Prepare function for abstract interpretation.
// Assumes the function is harnessed.
func (p prepAI) Function(fun *ssa.Function) func(tu.LoadResult) AnalysisCtxt {
	return func(loadRes tu.LoadResult) AnalysisCtxt {
		return p.prep(fun, fun, loadRes, true)
	}
}

// Find a function with the given name in the load result,
// and prepare it for abstract interpretation. Assumes the function
// is harnessed.
func (p prepAI) FunctionByName(name string, isHarnessed bool) func(tu.LoadResult) AnalysisCtxt {
	return func(loadRes tu.LoadResult) AnalysisCtxt {
		fun := loadRes.Cfg.FunctionByName(name)
		return p.prep(fun, fun, loadRes, isHarnessed)
	}
}

func (C AnalysisCtxt) IsPrimitiveFocused(prim ssa.Value) bool {
	if C.FocusedPrimitives == nil {
		return true
	}

	for _, foc := range C.FocusedPrimitives {
		if foc == prim {
			return true
		}
	}

	return false
}

// Computes a fragment predicate that includes all functions on paths to
// concurrency operations that use the provided primitives, including their allocation.
func (C *AnalysisCtxt) FragmentPredicateFromPrimitives(
	primitives []ssa.Value,
	primitiveToUses map[ssa.Value]map[*ssa.Function]struct{},
) {
	loadRes := C.LoadRes
	scc := loadRes.CallDAG

	interestingFunctions := map[*ssa.Function]bool{}

	for _, prim := range primitives {
		// The functions where the primitives are allocated are "interesting".
		interestingFunctions[prim.Parent()] = true

		// In addition to functions that use the primitive.
		for fun := range primitiveToUses[prim] {
			interestingFunctions[fun] = true
		}
	}

	included := make([]bool, len(scc.Components))

	for compIdx, component := range scc.Components {
		isInteresting := false

	OUTER:
		for _, node := range component {
			for _, edge := range scc.Original.Edges(node) {
				if included[scc.ComponentOf(edge)] {
					isInteresting = true
					break OUTER
				}
			}

			if interestingFunctions[node] {
				isInteresting = true
				break OUTER
			}
		}

		included[compIdx] = isInteresting
	}

	C.FocusedPrimitives = primitives

	C.FragmentPredicate = func(callIns ssa.CallInstruction, sfun *ssa.Function) bool {
		if idx := scc.ComponentOf(sfun); idx != -1 {
			return included[idx]
		}
		return false
	}
}
