package cfg

import (
	"errors"
	"fmt"
	"go/token"

	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

var (
	errUnsupportedNodeConversion = errors.New("unsupported CFG node type conversion")
)

type (
	// CallNodeRelation connects a pre-call CF-node to its post-call component.
	CallNodeRelation struct {
		post Node
	}

	// PostCallReation connects a post-call CF-node to its pre-call component.
	PostCallRelation struct {
		call Node
	}

	// CallRelation is implemented by both CallNodeRelation and PostCallRelation.
	CallRelation any

	// baseNode is the base type, embedded by all CF-nodes.
	baseNode struct {
		// deferred states whether a node maps the control flow when unwinding the defer stack.
		deferred bool
		// dfr connects the given node with its equivalent on the path to unwinding the defer stack.
		dfr Node
		// panicCont connects the given node with its continuation node in case of a panic.
		panicCont Node
		// panickers is a set of possible panic continuations.
		panickers map[Node]struct{}
		// spawn is a set of nodes denoting the initial control flow location of any any goroutines that may be spawned at the given instruction.
		spawn map[Node]struct{}
		// succ is a set of control flow successors.
		succ map[Node]struct{}
		// pred is a set of control flow predecessors.
		pred map[Node]struct{}
		// call encodes a call relation, connecting pre- and post-call nodes. Is nil for non-call nodes.
		call CallRelation
		// selfReaching is the cached result from computation checking whether a node is self-reachable.
		selfReaching *bool
	}

	// SSANode is a CF-node derived directly from an SSA instruction.
	SSANode struct {
		baseNode
		insn ssa.Instruction
	}

	// Node must be implemented by all possible CF-nodes.
	Node interface {
		// AddPredecessor adds a given node to the set of predecessors of the current node.
		AddPredecessor(Node)
		// removePredecessor removes a given node from the set of predecessors of the current node.
		removePredecessor(Node)
		// AddSuccessor adds a given node to the set of successors of the current node.
		AddSuccessor(Node)
		// removeSuccessor removes a given node from the set of successors of the current node.
		removeSuccessor(Node)
		// addSpawn adds a given node to the set of spawnees of the current node.
		addSpawn(Node)
		// addDeferLink sets up a defer connection between the given and current node.
		addDeferLink(Node)
		// addPanicCont adds a given node to the set of panic continuations of the current node.
		addPanicCont(Node)
		// addPanicker adds a given node to the set of pre-panic predecessors of the current node.
		addPanicker(Node)
		// setCallRelation updates the call relation information of the current node.
		setCallRelation(CallRelation)
		// setDeferred updates the current node as denoting part of the control flow when unwinding the defer stack.
		setDeferred()
		// base retrieves the underlying base node for any given node.
		base() *baseNode

		// Successor retrieves the successor of the given CF-node. Will panic if not strictly one possible successor exists.
		Successor() Node
		// Successors retrieves all the successors of the given CF-node.
		Successors() map[Node]struct{}
		// Spawn retrieves the entry node to a goroutine that may be spawned from the given CF-node. Will panic if not strictly one possible spawnee exists.
		Spawn() Node
		// Spawns retrieves all the possible entry nodes to a goroutine that may be spawned from the given CF-node.
		Spawns() map[Node]struct{}

		// Predecessor retrieves the predecessor of the given CF-node. Will panic if not strictly one possible predecessor exists.
		Predecessor() Node
		// Predecessors retrieves all the predecessors of the given CF-node.
		Predecessors() map[Node]struct{}
		// DeferLink retrieves the node connected via the defer instruction with the current node.
		DeferLink() Node
		// PanicCont retrieves the node that acts as a continuation in case of a run-time panic.
		PanicCont() Node
		// Continuations retrives the continuations of a node. If any regular successors
		// are present, return them. If not, fallback on the defer link or panic continuation.
		// If none of these are present, return the nil map.
		Continuations() map[Node]struct{}
		// Panickers retrieves all the nodes have this node as a
		// continuation in case of runtime panic.
		Panickers() map[Node]struct{}
		// IsDeferred checks whether the current node encodes part of the control flow when unwinding the defer stack.
		IsDeferred() bool
		// CallRelation retrieves the information for a call node i.e., a call node relation/post-call node relation.
		CallRelation() CallRelation
		// CallRelationNode retrieves the call-related CF-node i.e., pre/post-call node if the current is a post/pre-call node.
		CallRelationNode() Node

		// Function retrieves the parent function of the CF-node.
		Function() *ssa.Function
		// Block retrieves the basic block of the CF-node.
		Block() *ssa.BasicBlock

		// IsCommunicationNode checks whether the CF-node denotes a concurrency operation.
		IsCommunicationNode() bool
		// IsChannelOp checks whether the CF-node denotes a channel operation.
		IsChannelOp() bool
		// Channel retrieves the channel operand of a given CF-node. May panic if the node is not channel-related.
		Channel() ssa.Value
		// Mutex retrieves the mutex operand of a given CF-node. May panic if the node is not mutex-related.
		Mutex() ssa.Value
		// RWMutex retrieves the read-write mutex operand of a given CF-node. May panic if the node is not read-write mutex-related.
		RWMutex() ssa.Value
		// Locker retrieves the locker operand of a given CF-node. May panic if the node is not locker-related.
		Locker() ssa.Value
		// Cond retrieves the conditional variable operand of a given CF-node. May panic if the node is not conditional variable-related.
		Cond() ssa.Value
		// WaitGroup retrieves the waitgroup operand of a given CF-node. May panic if the node is not waitgroup-related.
		WaitGroup() ssa.Value
		// CommTransitive retrieves the nearest communication transitive successors of current node.
		// It returns the current node, if it represents a concurrency operation.
		CommTransitive() map[Node]struct{}

		// SSANode atempts conversion from the given node to a SSA-node.
		SSANode() *SSANode

		String() string

		// Pos retrieves the source location position of the CF-node.
		Pos() token.Pos
	}
)

// createSSANode constructs a fresh CF-node wrapping the SSA instruction.
func createSSANode(insn ssa.Instruction) (n *SSANode) {
	n = new(SSANode)
	n.succ = make(map[Node]struct{})
	n.pred = make(map[Node]struct{})
	n.spawn = make(map[Node]struct{})
	n.panickers = make(map[Node]struct{})
	n.insn = insn
	return
}

// base returns the current node itself, if it is already a base node.
func (n *baseNode) base() *baseNode {
	return n
}

// Successor retrieves the successor of the given CF-node. Will panic if not strictly one possible successor exists.
func (n *baseNode) Successor() Node {
	if len(n.succ) != 1 {
		panic("ERROR: Node does not have a single successor.")
	}
	for succ := range n.Successors() {
		return succ
	}
	return nil
}

func (n *baseNode) Successors() map[Node]struct{} {
	return n.succ
}

// Predecessor retrieves the predecessor of the given CF-node. Will panic if not strictly one possible predecessor exists.
func (n *baseNode) Predecessor() Node {
	if len(n.pred) != 1 {
		panic("ERROR: Node does not have a single predecessor.")
	}
	for pred := range n.Predecessors() {
		return pred
	}
	return nil
}

// IsChannelOp checks whether the CF-node denotes a channel operation.
func (n *baseNode) IsChannelOp() bool {
	return false
}

// Predecessors retrieves all the predecessors of the given CF-node.
func (n *baseNode) Predecessors() map[Node]struct{} {
	return n.pred
}

// Spawn retrieves the entry node to a goroutine that may be spawned from the given CF-node. Will panic if not strictly one possible spawnee exists.
func (n *baseNode) Spawn() Node {
	if len(n.spawn) > 1 {
		panic("ERROR: Node spawns more than one goroutine")
	}
	for spawn := range n.spawn {
		return spawn
	}
	return nil
}

// Spawns retrieves all the possible entry nodes to a goroutine that may be spawned from the given CF-node.
func (n *baseNode) Spawns() map[Node]struct{} {
	return n.spawn
}

// DeferLink retrieves the node connected via the defer instruction with the current node.
func (n *baseNode) DeferLink() Node {
	return n.dfr
}

// Panickers retrieves all the nodes have this node as a
// continuation in case of runtime panic.
func (n *baseNode) Panickers() map[Node]struct{} {
	return n.panickers
}

// Function retrieves the parent function of the CF-node.
func (n *SSANode) Function() *ssa.Function {
	return n.insn.Parent()
}

// Block retrieves the basic block of the CF-node.
func (n *SSANode) Block() *ssa.BasicBlock {
	return n.Instruction().Block()
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
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

// Channel retrieves the channel operand of a given CF-node. May panic if the node is not channel-related.
func (n *baseNode) Channel() ssa.Value {
	return nil
}

// Instruction retrieves the SSA instruction underlying an SSA derived CF-node.
func (n *SSANode) Instruction() ssa.Instruction {
	return n.insn
}

// callCommonIsConcurrent checks whether an SSA call instruction is
// a concurrent operation. The following return true.
//  1. (*sync.Mutex).{Lock, Unlock}
//  2. (*sync.RWMutex).{Lock, RLock, Unlock, RUnlock}
//  3. (sync.Locker).{Lock, Unlock}
//  4. (*sync.Cond).{Wait, Signal, Broadcast}
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
			// WaitGroup method call:
			case utils.IsNamedType(receiver, "sync", "WaitGroup") &&
				oneOf(sc.Name(), "Done", "Wait"):
				return true
			}

		case 2:
			receiver := cc.Args[0].Type()

			// WaitGroup method call:
			return utils.IsNamedType(receiver, "sync", "WaitGroup") && sc.Name() == "Add"
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

// IsCommunicationNode checks whether the CF-node denotes a concurrency operation.
func (n *SSANode) IsCommunicationNode() bool {
	switch i := n.Instruction().(type) {
	case *ssa.Call:
		return callCommonIsConcurrent(i.Call)
	default:
		return n.IsChannelOp()
	}
}

// IsChannelOp checks whether the CF-node denotes a channel operation.
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

// Channel retrieves the channel operand of a given CF-node. May panic if the node is not channel-related.
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

// getMutex retrieves the SSA register of the mutex operand for a mutex-operating instruction.
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

// Mutex retrieves the mutex operand of a given CF-node. May panic if the node is not mutex-related.
func (n *SSANode) Mutex() ssa.Value {
	return getMutex(n.Instruction())
}

// getRWMutex retrieves the SSA register of the read-write mutex operand of an SSA instruction.
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

// RWMutex retrieves the read-write mutex operand of a given CF-node. May panic if the node is not read-write mutex-related.
func (n *SSANode) RWMutex() ssa.Value {
	return getRWMutex(n.Instruction())
}

// getLocker extracts the locker SSA register of a given SSA instruction, or `nil`
// if the instruction does not operate on a locker value.
// It applies only to SSA values, or call instructions with a receiver,
// belonging to one of: sync.Mutex, sync.RWMutex or sync.Locker.
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

// Locker retrieves the locker value of a given CF-node derived from an SSA instruction.
func (n *SSANode) Locker() ssa.Value {
	return getLocker(n.Instruction())
}

// getCond extracts the conditional value SSA register of a given SSA instruction, or `nil`
// if the instruction does not operate on a conditional value.
// It applies only to SSA values, or call instructions with a receiver,
// belonging to one of: sync.Cond.
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

// Cond retrieves the conditional variable from the SSA instruction underlying
// the CF-node.
func (n *SSANode) Cond() ssa.Value {
	return getCond(n.Instruction())
}

func getWaitGroup(n ssa.Instruction) ssa.Value {
	isWaitGroup := func(v ssa.Value) bool {
		return utils.IsNamedType(v.Type(), "sync", "WaitGroup")
	}

	switch i := n.(type) {
	case ssa.CallInstruction:
		args := i.Common().Args
		if sc := i.Common().StaticCallee(); sc != nil && len(args) >= 1 && isWaitGroup(args[0]) {
			switch {
			case len(args) == 1 && (sc.Name() == "Done" || sc.Name() == "Wait"),
				len(args) == 2 && sc.Name() == "Add":
				return args[0]
			}
		}
	case ssa.Value:
		if isWaitGroup(i) {
			return i
		}
	}
	return nil
}

func (n *SSANode) WaitGroup() ssa.Value {
	return getWaitGroup(n.Instruction())
}

func (n *SSANode) String() string {
	switch i := n.insn.(type) {
	case ssa.Value:
		return fmt.Sprintf("[ %s = %s ]", i.Name(), i)
	default:
		return fmt.Sprintf("[ %s ]", i)
	}
}

// SSANode can be safely converted to SSANode.s
func (n *SSANode) SSANode() *SSANode {
	return n
}

// AddSuccessor adds a given node as a successor to the current node.
func (n *baseNode) AddSuccessor(n2 Node) {
	n.succ[n2] = struct{}{}
}

// removeSuccessor removes the given node from the set of successors of the current node.
func (n *baseNode) removeSuccessor(n2 Node) {
	delete(n.succ, n2)
}

// AddPredecessor adds a given node to the set of predecessors of the current node.
func (n *baseNode) AddPredecessor(n2 Node) {
	n.pred[n2] = struct{}{}
}

// removePredecessor removes a given node from the set of predecessors of the current node.
func (n *baseNode) removePredecessor(n2 Node) {
	delete(n.pred, n2)
}

// addSpawn adds a given node to the set of spawnees of the current node.
func (n *baseNode) addSpawn(n2 Node) {
	n.spawn[n2] = struct{}{}
}

// addDeferLink sets up a defer connection between the given and current node.
func (n *baseNode) addDeferLink(n2 Node) {
	n.dfr = n2
}

// addPanicker adds a given node to the set of pre-panic predecessors of the current node.
func (n *baseNode) addPanicker(n2 Node) {
	n.panickers[n2] = struct{}{}
}

// PanicCont retrieves the node that acts as a continuation in case of a run-time panic.
func (n *baseNode) PanicCont() Node {
	return n.panicCont
}

// addPanicCont adds a given node to the set of panic continuations of the current node.
func (n1 *baseNode) addPanicCont(n2 Node) {
	n1.panicCont = n2
}

// Continuations retrives the continuations of a node. If any regular successors
// are present, return them. If not, fallback on the defer link or panic continuation.
// If none of these are present, return the nil map.
func (n *baseNode) Continuations() (cont map[Node]struct{}) {
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

// setCallRelation updates the call relation information of the current node.
func (n *baseNode) setCallRelation(b CallRelation) {
	n.call = b
}

// CallRelation retrieves the information for a call node i.e., a call node relation/post-call node relation.
func (n *baseNode) CallRelation() CallRelation {
	return n.call
}

// CallRelationNode retrieves the call-related CF-node i.e., pre/post-call node if the current is a post/pre-call node.
func (n *baseNode) CallRelationNode() Node {
	switch r := n.call.(type) {
	case *PostCallRelation:
		return r.call
	case *CallNodeRelation:
		return r.post
	default:
		return nil
	}
}

// Pos returns the position of the underlying SSA instruction.
func (n *SSANode) Pos() token.Pos {
	return n.insn.Pos()
}

// SSANode is unsupported for base nodes.
func (n *baseNode) SSANode() *SSANode {
	panic(errUnsupportedNodeConversion)
}

// setDefer creates a defer link between the two nodes, and
// sets the second node as deferred.
func setDefer(n1 Node, n2 Node) {
	n2.setDeferred()
	n1.addDeferLink(n2)
	n2.addDeferLink(n1)
}

// setPanicCont sets the second node as a panic continuation of the first.
func setPanicCont(n1 Node, n2 Node) {
	n1.addPanicCont(n2)
	n2.addPanicker(n1)
}

// SetSuccessor sets the `to` node as a successor of the `from`, and
// the corresponding predecessor relation.
func SetSuccessor(from Node, to Node) {
	from.AddSuccessor(to)
	to.AddPredecessor(from)
}

// setCall sets up pre-/post-call relation between the `call` and `post` nodes.
func setCall(call Node, post Node) {
	bkcall := new(CallNodeRelation)
	bkcall.post = post
	bkpost := new(PostCallRelation)
	bkpost.call = call
	call.setCallRelation(bkcall)
	post.setCallRelation(bkpost)
}

// setSpawn sets the first node as possibly spawning a goroutine starting at the second node.
func setSpawn(n1 Node, n2 Node) {
	n1.addSpawn(n2)
}

// IsDeferred checks whether the current node encodes part of the control flow when unwinding the defer stack.
func (n *baseNode) IsDeferred() bool {
	return n.deferred
}

// setDeferred updates the current node as denoting part of the control flow when unwinding the defer stack.
func (n *baseNode) setDeferred() {
	n.deferred = true
}
