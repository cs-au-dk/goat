package defs

import (
	"fmt"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

type ctrlocContext struct {
	root *ssa.Function
	// True if the goroutine is in a panicked state
	panicked bool
	// True if the goroutine is unwinding its defer stack due to runtime.Goexit
	exiting bool
}

// NOTE: This struct is used as a map key in abs-config. It is therefore
// important that it can be correctly compared with ==.
type CtrLoc struct {
	node    cfg.Node
	context ctrlocContext
}

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

func (cl CtrLoc) Node() cfg.Node {
	return cl.node
}

func (cl CtrLoc) Context() ctrlocContext {
	return cl.context
}

func (cl CtrLoc) Root() *ssa.Function {
	return cl.context.root
}

func (cl CtrLoc) Panicked() bool {
	return cl.context.panicked
}

func (cl CtrLoc) Exiting() bool {
	return cl.context.exiting
}

func (cl CtrLoc) WithExiting(exiting bool) CtrLoc {
	cl.context.exiting = exiting
	return cl
}

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

func bhash(b bool) uint32 {
	if b {
		return 0x9e3779b9
	} else {
		return 0xdeadbeef
	}
}

func (cl CtrLoc) Hash() uint32 {
	phasher := utils.PointerHasher[any]{}
	return utils.HashCombine(
		phasher.Hash(cl.node),
		phasher.Hash(cl.context.root),
		bhash(cl.context.panicked),
		bhash(cl.context.exiting),
	)
}

// A control location is forking if it has more than one successor or predecessor.
func (cl CtrLoc) Forking() bool {
	return (len(cl.Successors()) > 1 || len(cl.Node().Predecessors()) > 1)
}

func (cl CtrLoc) String() string {
	var str string
	str += cl.node.String()
	// if cl.Root() != nil {
	// 	str = cl.Root().String() + ": " + str
	// }
	if cl.Panicked() {
		str += "(!)"
	}
	if cl.Exiting() {
		str += "[⇓]"
	}
	return str
}

// Derive new control location from current one with given control flow node.
func (cl CtrLoc) Derive(n cfg.Node) CtrLoc {
	return CtrLoc{
		n,
		cl.context,
	}
}

// Derive a batch of control locations from the provided set of control flow nodes.
func (cl CtrLoc) DeriveBatch(mp map[cfg.Node]struct{}) map[CtrLoc]bool {
	res := make(map[CtrLoc]bool)
	for n := range mp {
		res[cl.Derive(n)] = true
	}
	return res
}

// Derive control location successor from control flow node successor.
// Will panic if there is more than one successor.
func (cl CtrLoc) Successor() CtrLoc {
	return CtrLoc{
		cl.node.Successor(),
		cl.context,
	}
}

// Derive control location predecessor from control flow node predecessor
// Will panic if there is more than one predecessor.
func (cl CtrLoc) Predecessor() CtrLoc {
	return CtrLoc{
		cl.node.Predecessor(),
		cl.context,
	}
}

// Derive control location from the call relation node of the given node.
func (cl CtrLoc) CallRelationNode() CtrLoc {
	if crn := cl.Node().CallRelationNode(); crn != nil {
		return cl.Derive(crn)
	}
	panic(
		fmt.Sprintf("deriving call relation node for non-call related control location %s", cl),
	)
}

// Derive the panic continuation from control flow defer link,
// or from the given panic continuation.
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

// Derive control location successors through control flow node.
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

func (c1 CtrLoc) Equal(c2 CtrLoc) bool {
	return c1.node == c2.node && c1.context == c2.context
}
