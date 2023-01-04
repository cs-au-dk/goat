package leaf

import (
	"fmt"
	"go/token"

	"github.com/cs-au-dk/goat/analysis/cfg"
	loc "github.com/cs-au-dk/goat/analysis/location"

	"golang.org/x/tools/go/ssa"
)

type (
	// CommSend is a communication leaf where the channel allocated at Loc
	// attempts to perform a send operation. The CF-node set as the predecessor to
	// a CommSend leaf represents the source location.
	CommSend struct {
		cfg.Synthetic
		Loc loc.AddressableLocation
	}

	// CommRcv is a communication leaf where the channel allocated at Loc
	// attempts to perform a receive operation. The CF-node set as the predecessor to
	// a CommRcv leaf represents the source location.
	CommRcv struct {
		cfg.Synthetic
		Loc loc.AddressableLocation
	}
)

// Receiver returns the receiver of a CommRcv leaf as follows:
//
//  1. If the operation is a regular receive operation, the two scenarios are:
//     a) x = <-ch, in which case Receiver returns x, nil, false
//     b) x, ok = <-ch, in which case Receiver returns x, ok, true
//
//  2. If the operation is a receive case in a Select statement,
//     it returns the receiver value and ok value, if present, and false to indicate
//     that no tuple extraction is necessary.
func (n *CommRcv) Receiver() (ssa.Value, ssa.Value, bool) {
	switch n := n.Predecessor().(type) {
	case *cfg.SSANode:
		i, ok := n.Instruction().(*ssa.UnOp)
		if !ok || i.Op != token.ARROW {
			panic(fmt.Sprintf("CommSend predecessor node is not a channel receive? %v %T", n, n))
		}
		return i, nil, i.CommaOk
	case *cfg.SelectRcv:
		return n.Val, n.Ok, false
	default:
		panic(fmt.Sprintf("CommSend predecessor node is not a channel receive? %v %T", n, n))
	}
}

// Payload returns the value being sent to the channel in a CommSend leaf,
// by checking whether the send operation is a regular send statement or a send case
// in a select statement.
func (n *CommSend) Payload() ssa.Value {
	switch n := n.Predecessor().(type) {
	case *cfg.SSANode:
		i, ok := n.Instruction().(*ssa.Send)
		if !ok {
			panic(fmt.Sprintf("CommSend predecessor node is not a channel send? %v %T", n, n))
		}
		return i.X
	case *cfg.SelectSend:
		return n.Val
	default:
		panic(fmt.Sprintf("CommSend predecessor node is not a channel send? %v %T", n, n))
	}
}

// CommaOk indicates whether a receive operation extracts only a value, or if the
// also returns the "ok" boolean checking whether the value was extracted from a closed
// and empty channel
func (n *CommRcv) CommaOk() bool {
	switch n := n.Predecessor().(type) {
	case *cfg.SSANode:
		i, ok := n.Instruction().(*ssa.UnOp)
		if !ok || i.Op != token.ARROW {
			panic(fmt.Sprintf("CommSend predecessor node is not a channel receive? %v %T", n, n))
		}
		return i.CommaOk
	case *cfg.SelectRcv:
		return n.Ok != nil
	default:
		panic(fmt.Sprintf("CommSend predecessor node is not a channel receive? %v %T", n, n))
	}
}

// Pos invokes the Pos method of the underlying CF-node
func (n *CommSend) Pos() token.Pos {
	return n.Predecessor().Pos()
}

// Pos invokes the Pos method of the underlying CF-node
func (n *CommRcv) Pos() token.Pos {
	return n.Predecessor().Pos()
}

func (n *CommSend) String() string {
	return "[ leaf " + n.Loc.String() + "<- ]"
}

func (n *CommRcv) String() string {
	return "[ leaf <-" + n.Loc.String() + " ]"
}
