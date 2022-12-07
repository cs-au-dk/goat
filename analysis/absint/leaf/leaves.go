package leaf

import (
	"fmt"
	"go/token"
	"log"

	"github.com/cs-au-dk/goat/analysis/cfg"
	loc "github.com/cs-au-dk/goat/analysis/location"

	"golang.org/x/tools/go/ssa"
)

type CommSend struct {
	cfg.Synthetic
	Loc loc.AddressableLocation
}
type CommRcv struct {
	cfg.Synthetic
	Loc loc.AddressableLocation
}

// Returns receiver of communication node.
// If the predecessor is a select receive node, it returns the receiver value, ok value, if
// present, and a boolean indicating the receiver has been split.
// If the predecessor is a regular receive, it returns the receiver value, which
// will be a tuple from which the result payload, and "ok" will be later
// extracted.
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

func (n *CommSend) Pos() token.Pos {
	return n.Predecessor().Pos()
}

func (n *CommRcv) Pos() token.Pos {
	return n.Predecessor().Pos()
}

func (n *CommSend) String() string {
	return "[ concrete " + n.Loc.String() + " <- ]"
}

func (n *CommRcv) String() string {
	return "[ concrete <-" + n.Loc.String() + " ]"
}

type MuLock struct {
	cfg.Synthetic
	Loc loc.Location
}
type MuUnlock struct {
	cfg.Synthetic
	Loc loc.Location
}
type RWMuRLock struct {
	cfg.Synthetic
	Loc loc.Location
}
type RWMuRUnlock struct {
	cfg.Synthetic
	Loc loc.Location
}
type condLeafSynthetic struct {
	cfg.Synthetic
	Cnd loc.Location
}
type CondWaiting struct {
	condLeafSynthetic
}
type CondWaking struct {
	condLeafSynthetic
}
type CondWait struct {
	condLeafSynthetic
}
type CondSignal struct {
	condLeafSynthetic
}
type CondBroadcast struct {
	condLeafSynthetic
}

func (n *condLeafSynthetic) Cond() ssa.Value {
	return n.Predecessor().Cond()
}

func CreateLeaf(config cfg.SynthConfig, l loc.Location) cfg.Node {
	var n cfg.AnySynthetic
	switch config.Type {
	case cfg.SynthTypes.COMM_SEND:
		n = new(CommSend)
		if l == nil {
			log.Fatalln("nil location when constructing CommSend")
		}

		n.(*CommSend).Loc = l.(loc.AddressableLocation)
	case cfg.SynthTypes.COMM_RCV:
		n = new(CommRcv)
		if l == nil {
			log.Fatalln("nil location when constructing CommRcv")
		}

		n.(*CommRcv).Loc = l.(loc.AddressableLocation)
		//config.IdSuffixes = append([]string{l.String() + "<-"}, config.IdSuffixes...)
	case cfg.SynthTypes.LOCK:
		n = new(MuLock)
		n.(*MuLock).Loc = l
	case cfg.SynthTypes.UNLOCK:
		n = new(MuUnlock)
		n.(*MuUnlock).Loc = l
	case cfg.SynthTypes.RWMU_RLOCK:
		n = new(RWMuRLock)
		n.(*RWMuRLock).Loc = l
	case cfg.SynthTypes.RWMU_RUNLOCK:
		n = new(RWMuRUnlock)
		n.(*RWMuRUnlock).Loc = l
	case cfg.SynthTypes.COND_WAIT:
		n = new(CondWait)
		n.(*CondWait).Cnd = l
	case cfg.SynthTypes.COND_WAITING:
		n = new(CondWaiting)
		n.(*CondWaiting).Cnd = l
	case cfg.SynthTypes.COND_WAKING:
		n = new(CondWaking)
		n.(*CondWaking).Cnd = l
	case cfg.SynthTypes.COND_SIGNAL:
		n = new(CondSignal)
		n.(*CondSignal).Cnd = l
	case cfg.SynthTypes.COND_BROADCAST:
		n = new(CondBroadcast)
		n.(*CondBroadcast).Cnd = l
	default:
		panic(fmt.Errorf("Unsupported type: %d", config.Type))
	}
	// We're not adding the leaves to the global CFG, so we don't need an ID
	n.Init(config, "")
	return n
}
