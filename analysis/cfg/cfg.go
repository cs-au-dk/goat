package cfg

import (
	"fmt"
	"go/token"

	"github.com/cs-au-dk/goat/pkgutil"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

// funEntry is used for book-keeping information about a function in the program.
// It exposes entry/exit nodes and a set of CF-nodes within the body of the function.
type funEntry = struct {
	nodes       map[Node]struct{}
	entry, exit Node
}

// Cfg is the further instrumented control flow graph of a given program.
type Cfg struct {
	// fset is the file set of the original program. It is used for deriving
	// readable strings from CF-node positions.
	fset *token.FileSet

	// entries is the set of nodes that may act as entry points to the CFG.
	entries map[Node]struct{}
	// insnToNode maps SSA instructions to CF-nodes. Is the inversion of nodeToInsn
	insnToNode map[ssa.Instruction]Node
	// nodeToInsn maps CF-nodes to SSA instructions. Is the inversion of insnToNode
	nodeToInsn map[Node]ssa.Instruction
	// synthetics maps IDs as strings to synthetic CF-nodes.
	synthetics map[string]Node

	// funs connects SSA functions to their book-keeping information.
	funs map[*ssa.Function]funEntry
}

// init initializes a CFG, by initializing all required maps.
func (cfg *Cfg) init() {
	cfg.entries = make(map[Node]struct{})
	cfg.insnToNode = make(map[ssa.Instruction]Node)
	cfg.nodeToInsn = make(map[Node]ssa.Instruction)
	cfg.synthetics = make(map[string]Node)
	cfg.funs = make(map[*ssa.Function]funEntry)
}

// FileSet extracts the FileSet from the CFG.
func (cfg *Cfg) FileSet() *token.FileSet {
	return cfg.fset
}

// GetSynthetic retrieves a synthetic node given a synthetic node
// configuration, if it already exists in the CFG, or nil otherwise.
func (cfg *Cfg) GetSynthetic(config SynthConfig) Node {
	id := syntheticId(config)
	if node, ok := cfg.synthetics[id]; ok {
		return node
	}
	return nil
}

// ForEach executes the given procedure for each node in the CFG.
// Node traversal is performed in depth-first order starting at the entry
// nodes in an arbitrary order, with arbitrary ordering between siblings.
// The priority in the type of node relation edge during traversal is as follows:
//
//	Successor > Spawn > Panic continuation > Defer link edge
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

// FindAll aggregates all CF-nodes that satisfy the given predicate.
func (cfg *Cfg) FindAll(pred func(Node) bool) map[Node]struct{} {
	found := make(map[Node]struct{})

	cfg.ForEach(func(n Node) {
		if pred(n) {
			found[n] = struct{}{}
		}
	})

	return found
}

// addNode creates an SSA instruction equivalent CF-node, and adds it
// to the CFG if it did not previously exist. It returns the
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

// addSynthetic creates a synthetic CF-node given the configuration, and
// adds it to the CFG if it did not previously exist. It returns true for
// the `new` return variable, if the synthetic CF-node is newly added,
// and false otherwise.
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

// addEntry registers a node as a CFG entry point.
func (cfg *Cfg) addEntry(n Node) {
	cfg.entries[n] = struct{}{}
}

// GetEntries retrieves all CFG entry points in a slice, in arbitrary order.
func (cfg *Cfg) GetEntries() (ret []Node) {
	ret = make([]Node, 0, len(cfg.entries))
	for node := range cfg.entries {
		ret = append(ret, node)
	}

	return
}

// Functions aggregates all reachable functions from the CFG.
func (cfg *Cfg) Functions() map[*ssa.Function]struct{} {
	res := make(map[*ssa.Function]struct{})

	for fun := range cfg.funs {
		res[fun] = struct{}{}
	}

	return res
}

// FunIO yields the entry and exit CF-nodes of a given function.
func (cfg *Cfg) FunIO(f *ssa.Function) (entry Node, exit Node) {
	if fe, ok := cfg.funs[f]; ok {
		return fe.entry, fe.exit
	}
	return
}

// FunctionByName retrieves an SSA function by name. The search strategy is:
//
//  1. Attempt a fully qualified match
//  2. Match a function local to the targeted package by name
//  3. Match any function across all loaded package by name. This strategy is non-deterministic.
//
// If no function is found, FunctionByName will panic.
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

// SequentiallySelfReaching checks whether a node can reach itself in
// the CFG by following only successor edges.
func SequentiallySelfReaching(start Node) bool {
	base := start.base()
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

// CommunicationPrimitivesOf returns a list of communication primitives used in the node.
// The only (supposed) case where there may be multiple primitives is for cfg.Select nodes.
func CommunicationPrimitivesOf(node Node) (res []ssa.Value) {
	if !node.IsCommunicationNode() {
		return
	}

	switch node := node.(type) {
	case *TerminateGoro:
		// Termination nodes have no communication primitives
	case *BuiltinCall:
		// Calls to builtins `close` and `len` have their channel argument as the given communication primitives.
		res = []ssa.Value{node.Channel()}
	case *Waiting, *Waking:
		// Waiting and waking nodes
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
			node.WaitGroup(),
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
			node.WaitGroup(),
		} {
			if val != nil {
				res = append(res, val)
			}
		}
	}

	return
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

// CalleeCount counts how many call sites exist for each number of possible callees at a call site.
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

// CallerCount counts for each number of possible callers, how many function exit nodes may have that many callers.
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

// ChanOpsPointsToSets counts how many instructions exist for each possible points-to set size
// of channel operands in channel operation instructions.
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

	// Remove the number of instructions where no channel operand exists.
	delete(count, 0)
	return
}

// CheckImpreciseChanOps calculates channel imprecision by counting for each maximal
// points-to set how many channels exist. The maximal points-to set of a channel allocation
// is determined by finding the largest points-to set of a channel operand that includes the
// targeted channel allocation across all reachable channel operations.
func (cfg *Cfg) CheckImpreciseChanOps(pt *pointer.Result) (count map[int]int) {
	chs := make(map[ssa.Value]int)

	cfg.ForEach(func(n Node) {
		defer func() {
			recover()
		}()
		ch := n.Channel()
		if ch == nil {
			return
		}

		// We use labels as they are more precise, due to not ignoring the context sensitive heap.
		ptset := pt.Queries[ch].PointsTo().Labels()
		for _, l := range ptset {
			if och := l.Value(); och != nil && chs[och] < len(ptset) {
				chs[och] = len(ptset)
			}
		}
	})

	count = make(map[int]int)

	for _, maxptset := range chs {
		count[maxptset] += 1
	}

	// Remove the number of instructions where no channel operand exists.
	delete(count, 0)
	return
}
