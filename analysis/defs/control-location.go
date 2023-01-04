package defs

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

// ctrlocContext exposes contextual information about a control location.
// A control location's context include the root function of its goroutine's stack,
// and whether the program would be panicked during that execution, or unwinding
// its defer stack due to having called runtime.Goexit.
type ctrlocContext struct {
	root *ssa.Function
	// True if the goroutine is in a panicked state
	panicked bool
	// True if the goroutine is unwinding its defer stack due to runtime.Goexit
	exiting bool
}

// CtrLoc represents a control location in the analysis. It is used as a map key in abs-config,
// so it is important that it can be correctly compared with ==.
type CtrLoc struct {
	node    cfg.Node
	context ctrlocContext
}

// CtrLoc creates a a control location based on a node in the CFG, a root function, and
func (factory) CtrLoc(n cfg.Node, root *ssa.Function, panicked bool) CtrLoc {
	return CtrLoc{
		n,
		ctrlocContext{
			root,
			panicked,
			false,
		},
	}
}

// Node yields the node in a control location.
func (cl CtrLoc) Node() cfg.Node {
	return cl.node
}

// Context yields the control location context.
func (cl CtrLoc) Context() ctrlocContext {
	return cl.context
}

// Root yields the function called at the bottom of the stack of the control location's goroutine.
func (cl CtrLoc) Root() *ssa.Function {
	return cl.context.root
}

// Panicked yields whether the control location models an execution point in a panicked state.
func (cl CtrLoc) Panicked() bool {
	return cl.context.panicked
}

// Panicked yields whether the control location models an execution point in an exiting state.
func (cl CtrLoc) Exiting() bool {
	return cl.context.exiting
}

// WithExiting derives another control that inherits all the properties of the receiver
// control location, but its context `exiting` field changes to the given value.
func (cl CtrLoc) WithExiting(exiting bool) CtrLoc {
	cl.context.exiting = exiting
	return cl
}

// PosString returns a textual representation of the source location position for the
// given control location.
func (cl CtrLoc) PosString() string {
	var nearestPos func(cfg.Node) cfg.Node
	nearestPos = func(n cfg.Node) cfg.Node {
		if len(n.Predecessors()) != 1 {
			return n
		}

		if !n.Pos().IsValid() {
			return nearestPos(n.Predecessor())
		}
		f := n.Function()
		if f == nil {
			return nearestPos(n.Predecessor())
		}

		return n
	}

	n := nearestPos(cl.Node())
	if !n.Pos().IsValid() {
		return ""
	}
	f := n.Function()
	if f == nil {
		return ""
	}
	fset := f.Prog.Fset
	return fset.Position(n.Pos()).String()
}

// bhash computes a hash from a boolean value.
func bhash(b bool) uint32 {
	if b {
		return 0x9e3779b9
	} else {
		return 0xdeadbeef
	}
}

// Hash computes a 32-bit hash for control location based on all of its properties.
func (cl CtrLoc) Hash() uint32 {
	phasher := utils.PointerHasher[any]{}
	return utils.HashCombine(
		phasher.Hash(cl.node),
		phasher.Hash(cl.context.root),
		bhash(cl.context.panicked),
		bhash(cl.context.exiting),
	)
}

// Forking checks whether a control location has more than one successor or predecessor.
func (cl CtrLoc) Forking() bool {
	return (len(cl.Successors()) > 1 || len(cl.Node().Predecessors()) > 1)
}

func (cl CtrLoc) String() string {
	var str string
	str += cl.node.String()
	if cl.Panicked() {
		str += "(!)"
	}
	if cl.Exiting() {
		str += "[⇓]"
	}
	return str
}

// Derive a new control location, inheriting all the properties of the current control location,
// where the control flow node is overridden by the provided argument.
func (cl CtrLoc) Derive(n cfg.Node) CtrLoc {
	return CtrLoc{
		n,
		cl.context,
	}
}

// DeriveBatch derives a batch of new control location for each given node in the CFG,
// inheriting all properties of the receiver except the CF-node.
func (cl CtrLoc) DeriveBatch(mp map[cfg.Node]struct{}) map[CtrLoc]struct{} {
	res := make(map[CtrLoc]struct{})
	for n := range mp {
		res[cl.Derive(n)] = struct{}{}
	}
	return res
}

// Successor derives control location successor from the successor of the CF-node at the
// current control location. Will panic if there is not strictly one successor.
func (cl CtrLoc) Successor() CtrLoc {
	return CtrLoc{
		cl.node.Successor(),
		cl.context,
	}
}

// Predecessor derives control location predecessor from the successor of the CF-node at the
// current control location. Will panic if there is not strictly one predecessor.
func (cl CtrLoc) Predecessor() CtrLoc {
	return CtrLoc{
		cl.node.Predecessor(),
		cl.context,
	}
}

// CallRelationNode derives control location predecessor from the call-relation node of the CF-node at the
// current control location. Will panic if there the CF-node is a call-related node.
func (cl CtrLoc) CallRelationNode() CtrLoc {
	if crn := cl.Node().CallRelationNode(); crn != nil {
		return cl.Derive(crn)
	}
	panic(
		fmt.Sprintf("deriving call relation node for non-call related control location %s", cl),
	)
}

// Panic derives a panic continuation control location from control flow defer link,
// or from the panic continuation of the CF-node at the current control location.
func (cl CtrLoc) Panic() CtrLoc {
	cl.context.panicked = true
	switch {
	case cl.node.DeferLink() != nil:
		cl.node = cl.node.DeferLink()
	case cl.node.PanicCont() != nil:
		cl.node = cl.node.PanicCont()
	default:
		panic(fmt.Errorf("no panic continuation for control location %s", cl))
	}

	return cl
}

// Successors derives a set of control location successors from all the successors
// of the CF-node at the current control location.
func (cl CtrLoc) Successors() map[CtrLoc]struct{} {
	succs := make(map[CtrLoc]struct{})

	for succ := range cl.node.Successors() {
		succs[CtrLoc{
			succ,
			// TODO: Context might require different handling when more information is
			// added
			cl.context,
		}] = struct{}{}
	}

	return succs
}

// Equal checks for structual equality between control locations.
func (c1 CtrLoc) Equal(c2 CtrLoc) bool {
	return c1.node == c2.node && c1.context == c2.context
}
