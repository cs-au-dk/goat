package leaf

import (
	"fmt"
	"log"

	"github.com/cs-au-dk/goat/analysis/cfg"
	loc "github.com/cs-au-dk/goat/analysis/location"
)

// CreateLeaf creates a communication leaf based on the given configuration,
// and allocation site.
//
// Given the format:
//
//	[Config type value] (Leaf): Description
//
// The types of communication leaves are:
//
//	Channel communication leaves:
//	1. COMM_SEND        (CommSend):      Send communication operation
//
//	2. COMM_RCV         (CommRcv):       Receive communication operation
//
//	(Read-write) Mutex communication leaves:
//	3. LOCK             (MuLock):        (RW)Mutex lock operation
//
//	4. UNLOCK           (MuUnlock):      (RW)Mutex unlock operation
//
//	5. RWMU_RLOCK       (RWMuRLock):     RWMutex read-lock operation
//
//	6. RWMU_RUNLOCK     (RWMuRUnlock):   RWMutex read-unlock operation
//
//	Conditional variable communication leaves:
//	7. COND_WAIT        (CondWait):      Cond about to start waiting
//
//	8. COND_WAITING     (CondWaiting):   Cond started waiting
//
//	9. COND_WAKING      (CondWaking):    Cond is waking
//
//	10. COND_SIGNAL     (CondSignal):    Cond is signalling a random waiter to wake
//
//	11. COND_BROADCAST  (CondBroadcast): Cond is signalling all waiters to wake
//
//	12. WAITGROUP_ADD   (WaitGroupAdd):  WaitGroup is incremented with a delta
//
//	13. WAITGROUP_WAIT  (WaitGroupWait): WaitGroup about to start waiting
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
		n.(*CondWait).Loc = l
	case cfg.SynthTypes.COND_WAITING:
		n = new(CondWaiting)
		n.(*CondWaiting).Loc = l
	case cfg.SynthTypes.COND_WAKING:
		n = new(CondWaking)
		n.(*CondWaking).Loc = l
	case cfg.SynthTypes.COND_SIGNAL:
		n = new(CondSignal)
		n.(*CondSignal).Loc = l
	case cfg.SynthTypes.COND_BROADCAST:
		n = new(CondBroadcast)
		n.(*CondBroadcast).Loc = l
	case cfg.SynthTypes.WAITGROUP_ADD:
		n = new(WaitGroupAdd)
		n.(*WaitGroupAdd).Loc = l
	case cfg.SynthTypes.WAITGROUP_WAIT:
		n = new(WaitGroupWait)
		n.(*WaitGroupWait).Loc = l
	default:
		panic(fmt.Errorf("Unsupported type: %d", config.Type))
	}

	// We're not adding the leaves to the global CFG, so we don't need an ID
	n.Init(config, "")
	return n
}
