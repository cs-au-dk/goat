package cfg

import (
	"Goat/pkgutil"
	"fmt"
	"go/token"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

type funEntry = struct {
	nodes       map[Node]struct{}
	entry, exit Node
}

type Cfg struct {
	fset       *token.FileSet
	entries    map[Node]struct{}
	insnToNode map[ssa.Instruction]Node
	nodeToInsn map[Node]ssa.Instruction
	synthetics map[string]Node

	funs map[*ssa.Function]funEntry
}

func (cfg *Cfg) init() {
	cfg.entries = make(map[Node]struct{})
	cfg.insnToNode = make(map[ssa.Instruction]Node)
	cfg.nodeToInsn = make(map[Node]ssa.Instruction)
	cfg.synthetics = make(map[string]Node)
	cfg.funs = make(map[*ssa.Function]funEntry)
}

func (cfg *Cfg) HasNode(n Node) bool {
	_, ok := cfg.nodeToInsn[n]
	return ok
}

func (cfg *Cfg) FileSet() *token.FileSet {
	return cfg.fset
}

func (cfg *Cfg) GetNode(i ssa.Instruction) Node {
	if node, ok := cfg.insnToNode[i]; ok {
		return node
	}
	return nil
}

func (cfg *Cfg) GetSynthetic(config SynthConfig) Node {
	id := syntheticId(config)
	if node, ok := cfg.synthetics[id]; ok {
		return node
	}
	return nil
}

func (cfg *Cfg) HasInsn(i ssa.Instruction) bool {
	_, ok := cfg.insnToNode[i]
	return ok
}

func (cfg *Cfg) HasSynthetic(config SynthConfig) bool {
	id := syntheticId(config)
	_, ok := cfg.synthetics[id]
	return ok
}

func (cfg *Cfg) ForEach(do func(Node)) {
	visited := make(map[Node]struct{})

	var visit func(Node)
	visit = func(n Node) {
		if _, ok := visited[n]; !ok {
			visited[n] = struct{}{}

			do(n)

			for succ := range n.Successors() {
				visit(succ)
			}

			for spawn := range n.Spawns() {
				visit(spawn)
			}

			if pnc := n.PanicCont(); pnc != nil {
				visit(pnc)
			}
			if dfr := n.DeferLink(); dfr != nil {
				visit(dfr)
			}
		}
	}

	for entry := range cfg.entries {
		visit(entry)
	}
}

func (cfg *Cfg) ForEachFromIf(n Node, do func(Node), pred func(Node) bool) {
	visited := make(map[Node]struct{})

	var visit func(Node)
	visit = func(n Node) {
		if _, ok := visited[n]; !ok {
			visited[n] = struct{}{}

			if !pred(n) {
				return
			}

			do(n)

			for succ := range n.Successors() {
				visit(succ)
			}

			for spawn := range n.Spawns() {
				visit(spawn)
			}

			if pnc := n.PanicCont(); pnc != nil {
				visit(pnc)
			}
			if dfr := n.DeferLink(); dfr != nil {
				visit(dfr)
			}
		}
	}

	visit(n)
}

func (cfg *Cfg) ForEachFrom(n Node, do func(Node)) {
	cfg.ForEachFromIf(n, do, func(_ Node) bool { return true })
}

func (cfg *Cfg) FindAll(pred func(Node) bool) map[Node]struct{} {
	found := make(map[Node]struct{})

	cfg.ForEach(func(n Node) {
		if pred(n) {
			found[n] = struct{}{}
		}
	})

	return found
}

// Add SSA instruction node to the CFG, if it does not exist,
// then return it.
func (cfg *Cfg) addNode(i ssa.Instruction) Node {
	if i != nil {
		if _, ok := cfg.insnToNode[i]; !ok {
			if _, ok := cfg.funs[i.Parent()]; !ok {
				cfg.funs[i.Parent()] = funEntry{
					nodes: make(map[Node]struct{}),
				}
			}
			n := createSSANode(i)
			// Add the node to all relevant bookkeeping structures.
			cfg.insnToNode[i] = n
			cfg.nodeToInsn[n] = i
			cfg.funs[i.Parent()].nodes[n] = struct{}{}
		}
		return cfg.insnToNode[i]
	}
	return nil
}

// Add synthetic node to CFG. If a node with the same
// configuration already exists, it returns it and new as false.
func (cfg *Cfg) addSynthetic(config SynthConfig) (node Node, new bool) {
	// Compute a synthetic ID based on the configuration.
	id := syntheticId(config)
	if _, ok := cfg.synthetics[id]; !ok {
		// Compute node parent function as follows:
		var fun *ssa.Function
		switch {
		// If a function is present in the configuration, use it;
		case config.Function != nil:
			fun = config.Function
			// Otherwise use the parent of the provided block;
		case config.Block != nil:
			fun = config.Block.Parent()
			// Otherwise use the parent of the provided instruction;
		case config.Insn != nil:
			fun = config.Insn.Parent()
			// Otherwise the function remains nil.
		}
		new = true
		// If the CFG does not have a map of nodes for that functions,
		// create it and addd it to the map of functions of the CFG.
		if _, ok := cfg.funs[fun]; !ok {
			cfg.funs[fun] = funEntry{
				nodes: make(map[Node]struct{}),
			}
		}
		// Create synthetic node based on ID and configuration,
		// and add it to the relevant CFG substructures.
		n := createSynthetic(config, id)
		cfg.synthetics[id] = n
		cfg.funs[fun].nodes[n] = struct{}{}
	}
	return cfg.synthetics[id], new
}

// Add CFG entry point.
func (cfg *Cfg) addEntry(n Node) {
	cfg.entries[n] = struct{}{}
}

// Get all CFG entry points.
func (cfg *Cfg) GetEntries() (ret []Node) {
	ret = make([]Node, 0, len(cfg.entries))
	for node := range cfg.entries {
		ret = append(ret, node)
	}

	return
}

func (cfg *Cfg) Functions() map[*ssa.Function]struct{} {
	res := make(map[*ssa.Function]struct{})

	for fun := range cfg.funs {
		res[fun] = struct{}{}
	}

	return res
}

func (cfg *Cfg) FunIO(f *ssa.Function) (entry Node, exit Node) {
	if fe, ok := cfg.funs[f]; ok {
		return fe.entry, fe.exit
	}
	return
}

// Retrieve SSA function by name. Will attempt a fully qualified match first
// then fall back on a looser search and return the first matched function.
func (cfg *Cfg) FunctionByName(name string) *ssa.Function {
	funs := cfg.Functions()

	// First try a fully qualified match.
	for fun := range funs {
		if fun.String() == name {
			return fun
		}
	}

	// Match the first local function by name
	for fun := range funs {
		if pkgutil.IsLocal(fun) && fun.Name() == name {
			return fun
		}
	}

	// Match the first function by name
	for fun := range funs {
		if fun.Name() == name {
			return fun
		}
	}

	panic(
		fmt.Errorf("no function with the name %s was found", name))
}

// Retrieve all SSA functions which have the provided name.
// There is no attempt to fully qualify the function name, based on the
// containing package.
func (cfg *Cfg) AllFunctionsWithName(name string) map[*ssa.Function]struct{} {
	res := make(map[*ssa.Function]struct{})

	for fun := range cfg.Functions() {
		if fun.Name() == name {
			res[fun] = struct{}{}
		}
	}

	return res
}

func SequentiallySelfReaching(start Node) bool {
	base := start.baseNode()
	if base.selfReaching != nil {
		return *base.selfReaching
	}

	base.selfReaching = new(bool)
	visited := make(map[Node]struct{})

	var visit func(node Node)
	visit = func(node Node) {
		if _, ok := visited[node]; !ok {
			if start == node {
				*base.selfReaching = true
				return
			}
			visited[node] = struct{}{}
			for succ := range node.Successors() {
				visit(succ)
			}
		}
	}

	for succ := range start.Successors() {
		if !*base.selfReaching {
			visit(succ)
		}
	}
	return *base.selfReaching
}

// Returns a list of communication primitives used in the node.
// The only (supposed) case where there may be multiple primitives is for
// cfg.Select nodes.
func CommunicationPrimitivesOf(node Node) (res []ssa.Value) {
	if node.IsCommunicationNode() {
		switch node := node.(type) {
		case *TerminateGoro:
		case *BuiltinCall:
			res = []ssa.Value{node.Channel()}
		case *Waiting:
			res = []ssa.Value{node.Cond()}
		case *Waking:
			res = []ssa.Value{node.Cond()}
		case *Select:
			for _, op := range node.Ops() {
				if _, isDefault := op.(*SelectDefault); !isDefault {
					res = append(res, op.Channel())
				}
			}
		case *APIConcBuiltinCall:
			for _, val := range []ssa.Value{
				node.Locker(),
				node.Cond(),
			} {
				if val != nil {
					res = append(res, val)
				}
			}
		default:
			for _, val := range []ssa.Value{
				node.Channel(),
				node.Locker(),
				node.Cond(),
			} {
				if val != nil {
					res = append(res, val)
				}
			}
		}
	}

	return
}

var cfg *Cfg = &Cfg{}

func CFG() *Cfg {
	return cfg
}

func (cfg *Cfg) MaxCallees() (cs Node, maxCallees int) {
	update := func(n Node) {
		succs := len(n.Successors())

		if maxCallees < succs {
			maxCallees = succs
			cs = n
		}
	}

	cfg.ForEach(func(n Node) {
		switch n := n.(type) {
		case *SSANode:
			switch n.Instruction().(type) {
			case *ssa.Call:
				update(n)
			}
		case *DeferCall:
			update(n)
		}
	})

	return
}

// Count for each number of occurrences, how many call sites
// exist
func (cfg *Cfg) CalleeCount() (count map[int]int) {
	count = make(map[int]int)

	cfg.ForEach(func(n Node) {
		switch n := n.(type) {
		case *SSANode:
			switch n.Instruction().(type) {
			case *ssa.Call:
				count[len(n.Successors())] += 1
			}
		case *DeferCall:
			count[len(n.Successors())] += 1
		}
	})

	return
}

func (cfg *Cfg) CallerCount() (count map[int]int) {
	count = make(map[int]int)

	// Assumes CFG construction prunes unreachable nodes
	cfg.ForEach(func(n Node) {
		switch n := n.(type) {
		case *FunctionExit:
			count[len(n.Successors())] += 1
		}
	})

	return
}

func (cfg *Cfg) ChanOpsPointsToSets(pt *pointer.Result) (count map[int]int) {
	count = make(map[int]int)

	setSize := func(ch ssa.Value) int {
		return len(pt.Queries[ch].PointsTo().Labels())
	}

	cfg.ForEach(func(n Node) {
		defer func() {
			recover()
		}()

		count[setSize(n.Channel())] += 1
	})

	delete(count, 0)
	return
}

func (cfg *Cfg) CheckImpreciseChanOps(pt *pointer.Result) (count map[int]int) {
	chs := make(map[ssa.Value]int)

	// var prog *ssa.Program
	// makeSet := func(v ssa.Value) utils.SSAValueSet {
	// 	if prog == nil && v != nil && v.Parent() != nil {
	// 		prog = v.Parent().Prog
	// 	}

	// 	ptset := pt.Queries[v].PointsTo().Labels()
	// 	vs := make([]ssa.Value, 0, len(ptset))

	// 	for _, l := range ptset {
	// 		vs = append(vs, l.Value())
	// 	}
	// 	set := utils.MakeSSASet(vs...)

	// 	return set
	// }

	cfg.ForEach(func(n Node) {
		defer func() {
			recover()
		}()
		ch := n.Channel()
		if ch == nil {
			return
		}

		ptset := pt.Queries[ch].PointsTo().Labels()
		for _, l := range ptset {
			if och := l.Value(); och != nil && chs[och] < len(ptset) {
				chs[och] = len(ptset)
			}
		}

		// This variant is less precise (hence, more unsafe) than using labels, since
		// it ignores the context sensitive heap
		// ptset := makeSet(ch)

		// ptset.ForEach(func(och ssa.Value) {
		// 	if chs[och] < ptset.Size() {
		// 		chs[och] = ptset.Size()
		// 	}
		// })
	})

	count = make(map[int]int)

	for _, maxptset := range chs {
		count[maxptset] += 1
	}

	delete(count, 0)
	return
}
