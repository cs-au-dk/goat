package cfg

import (
	"errors"
	"fmt"
	"go/token"
	"math/rand"

	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

var (
	errUnsupportedNodeConversion = errors.New("unsupported CFG node type conversion")
)

type CallNodeRelation struct {
	post Node
}

type PostCallRelation struct {
	call Node
}

type CallRelation interface{}

type BaseNode struct {
	deferred     bool
	dfr          Node
	panicCont    Node
	panickers    map[Node]struct{}
	spawn        map[Node]struct{}
	spawners     map[Node]struct{}
	succ         map[Node]struct{}
	pred         map[Node]struct{}
	call         CallRelation
	selfReaching *bool
}

type SSANode struct {
	BaseNode
	insn ssa.Instruction
}

type Node interface {
	AddPredecessor(Node)
	removePredecessor(Node)
	AddSuccessor(Node)
	removeSuccessor(Node)
	addSpawn(Node)
	addSpawner(Node)
	addDeferLink(Node)
	addPanicCont(Node)
	addPanicker(Node)
	setCallRelation(CallRelation)
	setDeferred()
	baseNode() *BaseNode

	Successor() Node
	RandSuccessor() Node
	Successors() map[Node]struct{}
	Spawn() Node
	RandSpawn() Node
	Spawns() map[Node]struct{}
	Spawners() map[Node]struct{}
	Predecessor() Node
	RandPredecessor() Node
	Predecessors() map[Node]struct{}
	DeferLink() Node
	// Node that acts as a continuation in case of a run-time panic.
	PanicCont() Node
	// The continuations of a node. If any regular successors are present, return
	// them. If not, fallback on the defer link or panic continuation. If none of
	// these are present, return the nil map.
	Continuations() map[Node]struct{}
	// Which nodes have this node as a
	// continuation in case of runtime panic.
	Panickers() map[Node]struct{}
	IsDeferred() bool
	CallRelation() CallRelation
	CallRelationNode() Node

	// Book-keeping
	Function() *ssa.Function
	Block() *ssa.BasicBlock

	IsCommunicationNode() bool
	IsChannelOp() bool
	Channel() ssa.Value
	Mutex() ssa.Value
	RWMutex() ssa.Value
	Locker() ssa.Value
	Cond() ssa.Value
	// Retrieve nearest communication transitive successors of current node.
	// If the current node itself is a concurrency-relevant node, it is the
	// only one returned. Does not include artificial nodes, like goroutine termination
	// or communication leaves.
	CommTransitive() map[Node]struct{}

	// Type conversion API
	SSANode() *SSANode

	String() string

	Pos() token.Pos
}

func createSSANode(insn ssa.Instruction) (n *SSANode) {
	n = new(SSANode)
	n.succ = make(map[Node]struct{})
	n.pred = make(map[Node]struct{})
	n.spawn = make(map[Node]struct{})
	n.spawners = make(map[Node]struct{})
	n.panickers = make(map[Node]struct{})
	n.insn = insn
	return
}

func (n *BaseNode) baseNode() *BaseNode {
	return n
}

/* Returns the successor node. Panics if there is not strictly one successor.
 */
func (n *BaseNode) Successor() Node {
	if len(n.succ) != 1 {
		panic("ERROR: Node does not have a single successor.")
	}
	for succ := range n.Successors() {
		return succ
	}
	return nil
}

/**
 * Returns a random successor
 */
func (n *BaseNode) RandSuccessor() Node {
	count := len(n.succ)
	if count == 0 {
		return nil
	}
	random := rand.Intn(count)
	index := 0
	for succ := range n.succ {
		if random == index {
			return succ
		}
		index++
	}
	panic("ERROR: Random successor index out of bounds")
}

func (n *BaseNode) Successors() map[Node]struct{} {
	return n.succ
}

/* Returns the predecessor node. Panics if there is not strictly one predecessor.
 */
func (n *BaseNode) Predecessor() Node {
	if len(n.pred) != 1 {
		panic("ERROR: Node does not have a single predecessor.")
	}
	for pred := range n.Predecessors() {
		return pred
	}
	return nil
}

/**
 * Returns a random predecessor
 */
func (n *BaseNode) RandPredecessor() Node {
	count := len(n.pred)
	if count == 0 {
		return nil
	}
	random := rand.Intn(count)
	index := 0
	for pred := range n.pred {
		if random == index {
			return pred
		}
		index++
	}
	panic("ERROR: Random predecessor index out of bounds")
}

func (n *BaseNode) IsChannelOp() bool {
	return false
}

func (n *BaseNode) Predecessors() map[Node]struct{} {
	return n.pred
}

func (n *BaseNode) Spawn() Node {
	if len(n.spawn) > 1 {
		panic("ERROR: Node spawns more than one goroutine")
	}
	for spawn := range n.spawn {
		return spawn
	}
	return nil
}

/**
 * Returns a random spawn
 */
func (n *BaseNode) RandSpawn() Node {
	count := len(n.spawn)
	if count == 0 {
		return nil
	}
	random := rand.Intn(count)
	index := 0
	for spawn := range n.spawn {
		if random == index {
			return spawn
		}
		index++
	}
	panic("ERROR: Random spawn index out of bounds")
}

func (n *BaseNode) Spawns() map[Node]struct{} {
	return n.spawn
}

func (n *BaseNode) Spawners() map[Node]struct{} {
	return n.spawners
}

func (n *BaseNode) DeferLink() Node {
	return n.dfr
}

func (n *BaseNode) Panickers() map[Node]struct{} {
	return n.panickers
}

func (n *SSANode) Block() *ssa.BasicBlock {
	return n.Instruction().Block()
}

func (n *SSANode) CommTransitive() map[Node]struct{} {
	succs := make(map[Node]struct{})
	visited := make(map[Node]struct{})

	var visit func(Node)
	visit = func(n Node) {
		if _, found := visited[n]; !found {
			visited[n] = struct{}{}
			if n.IsCommunicationNode() {
				succs[n] = struct{}{}
				return
			}

			for succ := range n.Successors() {
				visit(succ)
			}
			// Return a termination node if the node is a function exit
			// with no successors (helps with annotations on exit nodes)
			if len(n.Successors()) == 0 {
				_, ok := n.(*FunctionExit)
				if ok {
					term, _ := cfg.addSynthetic(SynthConfig{
						Type:             SynthTypes.TERMINATE_GORO,
						Function:         n.Function(),
						TerminationCause: GoroTermination.EXIT_ROOT,
					})
					succs[term] = struct{}{}
				}
			}

			if dfr := n.DeferLink(); len(n.Successors()) == 0 && !n.IsDeferred() && dfr != nil {
				visit(dfr)
			}
			if pnc := n.PanicCont(); len(n.Successors()) == 0 && pnc != nil {
				visit(pnc)
			}
		}
	}

	visit(n)

	return succs
}

func (n *BaseNode) Channel() ssa.Value {
	return nil
}

func (n *SSANode) Payload() ssa.Value {
	i, ok := n.insn.(*ssa.Send)
	if !ok {
		panic(fmt.Sprintf("Called payload on non-Send SSA Node? %v %T", n, n))
	}
	return i.X
}

func (n *SSANode) Function() *ssa.Function {
	return n.insn.Parent()
}

func (n *SSANode) Instruction() ssa.Instruction {
	return n.insn
}

func callCommonIsConcurrent(cc ssa.CallCommon) bool {
	// If mutexes are not modelled, skip this step.
	if opts.SkipSync() {
		return false
	}

	oneOf := func(name string, valid ...string) bool {
		for _, i := range valid {
			if i == name {
				return true
			}
		}
		return false
	}

	if sc := cc.StaticCallee(); sc != nil {
		switch len(cc.Args) {
		case 1:
			receiver := cc.Args[0].Type()

			switch {
			// Mutex method call:
			case utils.IsNamedType(receiver, "sync", "Mutex") &&
				oneOf(sc.Name(), "Lock", "Unlock"):
				return true
			// RWMutex method call:
			case utils.IsNamedType(receiver, "sync", "RWMutex") &&
				oneOf(sc.Name(), "Lock", "Unlock", "RLock", "RUnlock"):
				return true
			// Cond method call:
			case utils.IsNamedType(receiver, "sync", "Cond") &&
				oneOf(sc.Name(), "Signal", "Wait", "Broadcast"):
				return true
			}
		}
		return false
	}

	// Check for concurrency influencing interface calls
	switch len(cc.Args) {
	case 0:
		switch {
		// Locker dynamically dispatched method call:
		case utils.IsNamedType(cc.Value.Type(), "sync", "Locker") &&
			oneOf(cc.Method.Name(), "Lock", "Unlock"):
			return true
		}
	}
	return false
}

func (n *SSANode) IsCommunicationNode() bool {
	switch i := n.Instruction().(type) {
	case *ssa.Call:
		return callCommonIsConcurrent(i.Call)
	default:
		return n.IsChannelOp()
	}
}

func (n *SSANode) IsChannelOp() bool {
	switch i := n.Instruction().(type) {
	case *ssa.Send:
		return true
	case *ssa.UnOp:
		return i.Op == token.ARROW
	default:
		return false
	}
}

func (n *SSANode) Channel() ssa.Value {
	switch i := n.Instruction().(type) {
	case *ssa.Send:
		return i.Chan
	case *ssa.UnOp:
		if i.Op == token.ARROW {
			return i.X
		}
	}
	return nil
}

func getMutex(n ssa.Instruction) ssa.Value {
	isMutex := func(v ssa.Value) bool {
		return utils.IsNamedType(v.Type(), "sync", "Mutex")
	}

	switch i := n.(type) {
	case ssa.CallInstruction:
		if sc := i.Common().StaticCallee(); sc != nil &&
			len(i.Common().Args) == 1 && isMutex(i.Common().Args[0]) {
			return i.Common().Args[0]
		}
	case ssa.Value:
		if isMutex(i) {
			return i
		}
	}
	return nil

}

func (n *SSANode) Mutex() ssa.Value {
	return getMutex(n.Instruction())
}

func getRWMutex(n ssa.Instruction) ssa.Value {
	isRWMutex := func(v ssa.Value) bool {
		return utils.IsNamedType(v.Type(), "sync", "RWMutex")
	}

	switch i := n.(type) {
	case ssa.CallInstruction:
		if sc := i.Common().StaticCallee(); sc != nil &&
			len(i.Common().Args) == 1 && isRWMutex(i.Common().Args[0]) {
			return i.Common().Args[0]
		}
	case ssa.Value:
		if isRWMutex(i) {
			return i
		}
	}
	return nil
}

func (n *SSANode) RWMutex() ssa.Value {
	return getRWMutex(n.Instruction())
}

func getLocker(n ssa.Instruction) ssa.Value {
	isLocker := func(v ssa.Value) bool {
		return utils.IsNamedType(v.Type(), "sync", "Mutex") ||
			utils.IsNamedType(v.Type(), "sync", "RWMutex") ||
			utils.IsNamedType(v.Type(), "sync", "Locker")
	}

	switch i := n.(type) {
	case ssa.CallInstruction:
		if sc := i.Common().StaticCallee(); sc != nil {
			if utils.IsNamedType(i.Common().Args[0].Type(), "sync", "Mutex") ||
				utils.IsNamedType(i.Common().Args[0].Type(), "sync", "RWMutex") {
				return i.Common().Args[0]
			}
		} else {
			if utils.IsNamedType(i.Common().Value.Type(), "sync", "Locker") {
				return i.Common().Value
			}
		}
	case ssa.Value:
		if isLocker(i) {
			return i
		}
	}
	return nil
}

func (n *SSANode) Locker() ssa.Value {
	return getLocker(n.Instruction())
}

func getCond(n ssa.Instruction) ssa.Value {
	isCond := func(v ssa.Value) bool {
		return utils.IsNamedType(v.Type(), "sync", "Cond")
	}

	switch i := n.(type) {
	case ssa.CallInstruction:
		if sc := i.Common().StaticCallee(); sc != nil &&
			len(i.Common().Args) == 1 && isCond(i.Common().Args[0]) {
			return i.Common().Args[0]
		}
	case ssa.Value:
		if isCond(i) {
			return i
		}
	}
	return nil
}

func (n *SSANode) Cond() ssa.Value {
	return getCond(n.Instruction())
}

func (n *SSANode) String() string {
	switch i := n.insn.(type) {
	case ssa.Value:
		return fmt.Sprintf("[ %s = %s ]", i.Name(), i)
	default:
		return fmt.Sprintf("[ %s ]", i)
	}
}

func (n *SSANode) SSANode() *SSANode {
	return n
}

func (n *BaseNode) AddSuccessor(n2 Node) {
	n.succ[n2] = struct{}{}
}

func (n *BaseNode) removeSuccessor(n2 Node) {
	delete(n.succ, n2)
}

func (n *BaseNode) AddPredecessor(n2 Node) {
	n.pred[n2] = struct{}{}
}

func (n *BaseNode) removePredecessor(n2 Node) {
	delete(n.pred, n2)
}

func (n *BaseNode) addSpawn(n2 Node) {
	n.spawn[n2] = struct{}{}
}

func (n *BaseNode) addSpawner(n2 Node) {
	n.spawners[n2] = struct{}{}
}

func (n *BaseNode) addDeferLink(n2 Node) {
	n.dfr = n2
}

func (n *BaseNode) addPanicker(n2 Node) {
	n.panickers[n2] = struct{}{}
}

func (n *BaseNode) PanicCont() Node {
	return n.panicCont
}

func (n1 *BaseNode) addPanicCont(n2 Node) {
	n1.panicCont = n2
}

func (n *BaseNode) Continuations() (cont map[Node]struct{}) {
	if len(n.Successors()) > 0 {
		return n.Successors()
	}
	if dfr := n.DeferLink(); dfr != nil {
		cont = make(map[Node]struct{})
		cont[dfr] = struct{}{}
		return cont
	} else if pnc := n.PanicCont(); pnc != nil {
		cont = make(map[Node]struct{})
		cont[pnc] = struct{}{}
		return cont
	}
	return
}

func (n *BaseNode) setCallRelation(b CallRelation) {
	n.call = b
}

func (n *BaseNode) CallRelation() CallRelation {
	return n.call
}

func (n *BaseNode) CallRelationNode() Node {
	switch r := n.call.(type) {
	case *PostCallRelation:
		return r.call
	case *CallNodeRelation:
		return r.post
	default:
		return nil
	}
}

func (n *SSANode) Pos() token.Pos {
	return n.insn.Pos()
}

func (n *BaseNode) SSANode() *SSANode {
	panic(errUnsupportedNodeConversion)
}

func setDefer(n1 Node, n2 Node) {
	n2.setDeferred()
	n1.addDeferLink(n2)
	n2.addDeferLink(n1)
}

func setPanicCont(n1 Node, n2 Node) {
	n1.addPanicCont(n2)
	n2.addPanicker(n1)
}

func SetSuccessor(from Node, to Node) {
	from.AddSuccessor(to)
	to.AddPredecessor(from)
}

func setCall(call Node, post Node) {
	bkcall := new(CallNodeRelation)
	bkcall.post = post
	bkpost := new(PostCallRelation)
	bkpost.call = call
	call.setCallRelation(bkcall)
	post.setCallRelation(bkpost)
}

func setSpawn(n1 Node, n2 Node) {
	n1.addSpawn(n2)
	n2.addSpawner(n1)
}

func (n *BaseNode) IsDeferred() bool {
	return n.deferred
}

func (n *BaseNode) setDeferred() {
	n.deferred = true
}
