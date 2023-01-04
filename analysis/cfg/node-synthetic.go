package cfg

import (
	"fmt"
	"go/token"
	"go/types"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

// SYNTH_TYPE_ID is the type of synthetic type identifiers.
type SYNTH_TYPE_ID = int

// SynthTypes is a collection of IDs for synthetic node types.
// Wrapped in a struct to avoid excessively polluting the namespace.
var SynthTypes = struct {
	// Structurally
	BLOCK_ENTRY       SYNTH_TYPE_ID
	BLOCK_EXIT        SYNTH_TYPE_ID
	BLOCK_ENTRY_DEFER SYNTH_TYPE_ID
	BLOCK_EXIT_DEFER  SYNTH_TYPE_ID
	FUNCTION_ENTRY    SYNTH_TYPE_ID
	FUNCTION_EXIT     SYNTH_TYPE_ID
	TERMINATE_GORO    SYNTH_TYPE_ID
	COMM_SEND         SYNTH_TYPE_ID
	COMM_RCV          SYNTH_TYPE_ID
	DEFER_CALL        SYNTH_TYPE_ID
	POST_DEFER_CALL   SYNTH_TYPE_ID
	POST_CALL         SYNTH_TYPE_ID
	SELECT_SEND       SYNTH_TYPE_ID
	SELECT_RCV        SYNTH_TYPE_ID
	SELECT_DEFAULT    SYNTH_TYPE_ID
	SELECT            SYNTH_TYPE_ID
	SELECT_DEFER      SYNTH_TYPE_ID
	BUILTIN_CALL      SYNTH_TYPE_ID
	PENDING_GO        SYNTH_TYPE_ID
	LOCK              SYNTH_TYPE_ID
	UNLOCK            SYNTH_TYPE_ID
	RWMU_RLOCK        SYNTH_TYPE_ID
	RWMU_RUNLOCK      SYNTH_TYPE_ID
	WAITING           SYNTH_TYPE_ID
	WAKING            SYNTH_TYPE_ID
	COND_WAIT         SYNTH_TYPE_ID
	COND_WAITING      SYNTH_TYPE_ID
	COND_WAKING       SYNTH_TYPE_ID
	COND_SIGNAL       SYNTH_TYPE_ID
	COND_BROADCAST    SYNTH_TYPE_ID
	WAITGROUP_ADD     SYNTH_TYPE_ID
	WAITGROUP_WAIT    SYNTH_TYPE_ID
	API_CONC_BUILTIN  SYNTH_TYPE_ID
}{
	BLOCK_ENTRY:       0,
	BLOCK_EXIT:        1,
	BLOCK_ENTRY_DEFER: 2,
	BLOCK_EXIT_DEFER:  3,
	FUNCTION_ENTRY:    4,
	FUNCTION_EXIT:     5,
	TERMINATE_GORO:    6,
	COMM_SEND:         7,
	COMM_RCV:          8,
	DEFER_CALL:        9,
	POST_DEFER_CALL:   10,
	POST_CALL:         11,
	SELECT_SEND:       12,
	SELECT_RCV:        13,
	SELECT_DEFAULT:    14,
	SELECT:            15,
	SELECT_DEFER:      16,
	BUILTIN_CALL:      17,
	PENDING_GO:        18,
	LOCK:              19,
	UNLOCK:            20,
	RWMU_RLOCK:        23,
	RWMU_RUNLOCK:      24,
	WAITING:           25,
	WAKING:            26,
	COND_WAIT:         27,
	COND_WAITING:      28,
	COND_WAKING:       29,
	COND_SIGNAL:       30,
	COND_BROADCAST:    31,
	WAITGROUP_ADD:     32,
	WAITGROUP_WAIT:    33,
	API_CONC_BUILTIN:  34,
}

// Types of synthetic nodes.
type (
	// Synthetic is the basic implementation of synthetic nodes, which must embed it.
	Synthetic struct {
		baseNode

		// fun is the function to which the synthetic node belongs.
		fun *ssa.Function
		// block is the basic block to which the synthetic node belongs.
		block *ssa.BasicBlock
		// id is a unique identifier.
		id string
	}

	// BlockEntry is a synthetic node marking the entry to a block.
	BlockEntry struct{ Synthetic }
	// BlockExit is a synthetic node marking the exit from a block.
	BlockExit struct{ Synthetic }
	// BlockEntryDefer is a synthetic node marking the entry to a block when unwiding the defer stack.
	BlockEntryDefer struct{ Synthetic }
	// BlockExitDefer is a synthetic node marking the exit from a block when unwiding the defer stack.
	BlockExitDefer struct{ Synthetic }
	// FunctionEntry is a synthetic node marking the entry to function.
	FunctionEntry struct{ Synthetic }
	// FunctionEntry is a synthetic node marking the exit from a function.
	FunctionExit struct{ Synthetic }
	// PendingGo is TODO: unused?
	PendingGo struct{ Synthetic }
	// TerminateGoro is a synthetic node denoting that a goroutine is terminated.
	TerminateGoro struct {
		Synthetic
		cause _TERMINATION_CAUSE
	}
)

// _TERMINATION_CAUSE encodes the causes of goroutine termination.
type _TERMINATION_CAUSE = int

// GoroTermination enumerates the causes of goroutine termination.
//
//	EXIT_ROOT: the goroutine terminated by encountering the exit node of the function at its root.
//	PERMANENTLY_BLOCKED: the goroutine terminated by becoming permanently blocked due to a caseless select statement.
//	INFINITE_LOOP: the goroutine terminated by entering an infinite loop without communication operations.
var GoroTermination = struct {
	EXIT_ROOT           _TERMINATION_CAUSE
	PERMANENTLY_BLOCKED _TERMINATION_CAUSE
	INFINITE_LOOP       _TERMINATION_CAUSE
}{
	EXIT_ROOT:           0,
	PERMANENTLY_BLOCKED: 1,
	INFINITE_LOOP:       2,
}

type (
	// ChnSynthetic is implemented by all synthetic nodes involving channel makechan SSA values.
	ChnSynthetic interface {
		AnySynthetic
		Channel() ssa.Value
	}

	// Structure underlying any channel synthetic node.
	chnSynthetic struct {
		Synthetic
		chn ssa.Value
	}

	// selectChnSynthetic is a synthetic channel node specialized for select statements.
	selectChnSynthetic struct {
		chnSynthetic
		Parent *Select
		pos    token.Pos
	}
	// SelectSend is a synthetic channel node encoding a send case in a select statement.
	SelectSend struct {
		selectChnSynthetic
		Val ssa.Value
	}
	// SelectRcv is a synthetic channel node encoding a receive case in a select statement.
	SelectRcv struct {
		selectChnSynthetic
		Val ssa.Value
		Ok  ssa.Value
	}
	// SelectDefault is a synthetic channel node encoding a default case in a select statement.
	SelectDefault struct {
		selectChnSynthetic
	}

	// Select is a synthetic node encoding the entry to a select statement.
	Select struct {
		Synthetic
		ops  []ChnSynthetic
		Insn *ssa.Select
	}
	// SelectDefer is a synthetic node that maps the control flow underneath a select statement,
	// when unwinding the defer stack.
	SelectDefer struct{ Synthetic }
	// DeferCall is a synthetic node denoting the entry to a deferred function call.
	DeferCall struct{ Synthetic }
	// PostDeferCall is a synthetic node denoting the exit from a deferred function call.
	PostDeferCall struct{ Synthetic }
	// PostCall is a synthetic node that maps the control flow after exiting a call instruction.
	PostCall struct{ Synthetic }
	// BuiltinCall is a synthetic node representing a call to builtin function.
	BuiltinCall struct {
		Synthetic
		Call    ssa.CallInstruction
		builtin *ssa.Builtin
	}
	// Waiting is a synthetic node denoting that a goroutine has started waiting due to a Cond.Wait call.
	Waiting struct {
		Synthetic
		Call ssa.CallInstruction
	}
	// Waking is a synthetic node denoting that a goroutine has started waking from a Cond.Wait call after
	// being signalled by a corresponding Cond.Signal or Cond.Broadcast call.
	Waking struct {
		Synthetic
		Call ssa.CallInstruction
	}
	// APIConcBuiltinCall denotes invocations of the builtin concurrency primitive API.
	APIConcBuiltinCall struct {
		Synthetic
		Call ssa.CallInstruction
	}
)

// Channel retrieves the SSA value denoting the channel in a synthetic channel node.
func (n *chnSynthetic) Channel() ssa.Value {
	return n.chn
}

// SynthoConfig is a universal implementation of synthetic node configuration.
// Used when creating synthetic nodes. When instantiating, only the necessary
// fields need to be specified, depending on the type of the defined node.
type SynthConfig struct {
	// Mandatory: specifies the type of synthetic node.
	Type SYNTH_TYPE_ID
	// Function denotes the parent function
	Function *ssa.Function
	// Block denotes the parent basic block
	Block *ssa.BasicBlock
	// Insn is the associated instruction
	Insn ssa.Instruction
	// Chn is used for nodes related to channel values
	Chn *ssa.MakeChan
	// Call is used by nodes related to call instructions.
	Call ssa.CallInstruction
	// Vals denotes other locations which might be used are encoded as an arbitrary list of values.
	// The uses differ for each type of synthetic nodes, and are encoded as follows:
	// -- Builtin calls:
	//			0 - builtin function
	// -- Select send case nodes:
	//			0 - channel
	//			1 - sent value
	// -- Select receive case nodes:
	//			0 - channel
	//			1 - received value (if receive with assignment, e. g. x = <-c)
	//			2 - received ok (if receive with tuple assignment, e. g. x, ok = <-c)
	Vals []ssa.Value
	// SelectIndex is the index of case branch in a select statement
	SelectIndex int
	// SelectOps denotes all case branches of a synthetic select node
	SelectOps []ChnSynthetic
	// SelectParent denotes the parent select node of a select case branch node
	SelectParent *Select
	// Pos is the token encoding the appropriate source position of the synthetic node.
	Pos token.Pos
	// TerminationCause is used for synthetic termination nodes.
	TerminationCause _TERMINATION_CAUSE
	// IdSuffixes encodes all optional suffixes used for the synthetic node ID.
	IdSuffixes []string
}

// AnySynthetic is an interface that must be implemented by all synthetic nodes.
type AnySynthetic interface {
	// Extends the universal Node interface.
	Node
	// Synthetic node ID.
	Id() string
	// Parent basic block.
	Block() *ssa.BasicBlock
	// Initialization synthetic node structures,
	// e. g. successor/predecessor maps.
	Init(config SynthConfig, id string)
}

// Id retrieves the unique ID of a synthetic node.
func (n *Synthetic) Id() string {
	return n.id
}

// CreateSynthetic is the public API for creating synthetic nodes.
// Useful for creating stand-alone nodes outside the globally computed CFG.
func CreateSynthetic(config SynthConfig) (n Node) {
	// Create synthetic ID here (avoids recomputing later).
	return createSynthetic(config, syntheticId(config))
}

// AddSynthetic is a public wrapper around addSynthetic. Like `CreateSynthetic` it but does not
// generate a new node if an identical previous one exists.
func (cfg *Cfg) AddSynthetic(config SynthConfig) Node {
	node, _ := cfg.addSynthetic(config)
	return node
}

// createSynthetic creates a synthetic node, given the configuration and computed ID.
func createSynthetic(config SynthConfig, id string) Node {
	var n AnySynthetic
	// Allocate memory for a different type of synthetic node,
	// based on the type specified in the configuration.
	switch config.Type {
	case SynthTypes.BLOCK_ENTRY:
		n = new(BlockEntry)
	case SynthTypes.BLOCK_EXIT:
		n = new(BlockExit)
	case SynthTypes.BLOCK_ENTRY_DEFER:
		n = new(BlockEntryDefer)
	case SynthTypes.BLOCK_EXIT_DEFER:
		n = new(BlockExitDefer)
	case SynthTypes.TERMINATE_GORO:
		n = new(TerminateGoro)
		n.(*TerminateGoro).cause = config.TerminationCause
	case SynthTypes.FUNCTION_ENTRY:
		n = new(FunctionEntry)
	case SynthTypes.FUNCTION_EXIT:
		n = new(FunctionExit)
	case SynthTypes.POST_CALL:
		n = new(PostCall)
	case SynthTypes.DEFER_CALL:
		n = new(DeferCall)
	case SynthTypes.POST_DEFER_CALL:
		n = new(PostDeferCall)
	case SynthTypes.PENDING_GO:
		n = new(PendingGo)
	case SynthTypes.SELECT_SEND:
		n = new(SelectSend)
		n.(*SelectSend).chn = config.Vals[0]
		n.(*SelectSend).Val = config.Vals[1]
		n.(*SelectSend).Parent = config.SelectParent

		if !config.Pos.IsValid() {
			log.Fatalln("Position for SelectSend is invalid!")
		}
		n.(*SelectSend).pos = config.Pos
	case SynthTypes.SELECT_RCV:
		n = new(SelectRcv)
		n.(*SelectRcv).chn = config.Vals[0]
		switch {
		case len(config.Vals) > 2 && config.Vals[2] != nil:
			n.(*SelectRcv).Ok = config.Vals[2]
			fallthrough
		case len(config.Vals) > 1 && config.Vals[1] != nil:
			n.(*SelectRcv).Val = config.Vals[1]
		}
		n.(*SelectRcv).Parent = config.SelectParent

		if !config.Pos.IsValid() {
			log.Fatalln("Position for SelectRcv is invalid!")
		}

		n.(*SelectRcv).pos = config.Pos
	case SynthTypes.SELECT_DEFAULT:
		n = new(SelectDefault)
		n.(*SelectDefault).Parent = config.SelectParent
		n.(*SelectDefault).pos = config.Pos
	case SynthTypes.SELECT:
		n = new(Select)
		n.(*Select).ops = config.SelectOps
	case SynthTypes.SELECT_DEFER:
		n = new(SelectDefer)
	case SynthTypes.BUILTIN_CALL:
		n = new(BuiltinCall)
		n.(*BuiltinCall).Call = config.Call
		n.(*BuiltinCall).builtin = config.Vals[0].(*ssa.Builtin)
	case SynthTypes.WAITING:
		n = new(Waiting)
		n.(*Waiting).Call = config.Call
	case SynthTypes.WAKING:
		n = new(Waking)
		n.(*Waking).Call = config.Call
	case SynthTypes.API_CONC_BUILTIN:
		n = new(APIConcBuiltinCall)
		n.(*APIConcBuiltinCall).Call = config.Call
	default:
		log.Fatal("Inexhaustive pattern match: ", config.Type)
		os.Exit(1)
	}
	n.Init(config, id)
	return n.(Node)
}

// Construct a synthetic node ID, given a configuration.
func syntheticId(config SynthConfig) string {
	// Prepend ID suffixes with a string representing the type of synthetic node.
	switch config.Type {
	case SynthTypes.BLOCK_ENTRY:
		config.IdSuffixes = append([]string{"block-entry"}, config.IdSuffixes...)
	case SynthTypes.BLOCK_EXIT:
		config.IdSuffixes = append([]string{"block-exit"}, config.IdSuffixes...)
	case SynthTypes.BLOCK_ENTRY_DEFER:
		config.IdSuffixes = append([]string{"block-entry-defer"}, config.IdSuffixes...)
	case SynthTypes.BLOCK_EXIT_DEFER:
		config.IdSuffixes = append([]string{"block-exit-defer"}, config.IdSuffixes...)
	case SynthTypes.TERMINATE_GORO:
		suffix := ""
		if config.TerminationCause == GoroTermination.PERMANENTLY_BLOCKED {
			suffix = "-blocked"
		}
		config.IdSuffixes = append([]string{"terminate-goro" + suffix}, config.IdSuffixes...)
	case SynthTypes.FUNCTION_ENTRY:
		config.IdSuffixes = append([]string{"fun-entry"}, config.IdSuffixes...)
	case SynthTypes.FUNCTION_EXIT:
		config.IdSuffixes = append([]string{"fun-exit"}, config.IdSuffixes...)
	case SynthTypes.POST_CALL:
		config.IdSuffixes = append([]string{"post-call"}, config.IdSuffixes...)
	case SynthTypes.COMM_RCV:
		//config.IdSuffixes = append([]string{config.Loc.String() + "<-"}, config.IdSuffixes...)
	case SynthTypes.COMM_SEND:
		//config.IdSuffixes = append([]string{"<-" + config.Loc.String()}, config.IdSuffixes...)
	case SynthTypes.DEFER_CALL:
		config.IdSuffixes = append([]string{"pre-defer"}, config.IdSuffixes...)
	case SynthTypes.POST_DEFER_CALL:
		config.IdSuffixes = append([]string{"post-defer"}, config.IdSuffixes...)
	case SynthTypes.SELECT_SEND:
		config.IdSuffixes = append([]string{"select-" + strconv.Itoa(int(config.Insn.Pos())) + ":" + strconv.Itoa(config.SelectIndex) + "-" + config.Vals[0].Name() + "<-"}, config.IdSuffixes...)
	case SynthTypes.SELECT_RCV:
		config.IdSuffixes = append([]string{"select-" + strconv.Itoa(int(config.Insn.Pos())) + ":" + strconv.Itoa(config.SelectIndex) + "-<-" + config.Vals[0].Name()}, config.IdSuffixes...)
	case SynthTypes.SELECT_DEFAULT:
		config.IdSuffixes = append([]string{"select-default" + strconv.Itoa(int(config.Insn.Pos()))}, config.IdSuffixes...)
	case SynthTypes.SELECT:
		config.IdSuffixes = append([]string{"select" + strconv.Itoa(int(config.Insn.Pos()))}, config.IdSuffixes...)
	case SynthTypes.SELECT_DEFER:
		config.IdSuffixes = append([]string{"select-defer" + strconv.Itoa(int(config.Insn.Pos()))}, config.IdSuffixes...)
	case SynthTypes.BUILTIN_CALL:
		config.IdSuffixes = append([]string{"builtin"}, config.IdSuffixes...)
	case SynthTypes.PENDING_GO:
		config.IdSuffixes = append([]string{"pending-go"}, config.IdSuffixes...)
	case SynthTypes.LOCK:
		//config.IdSuffixes = append([]string{"mu-lock(" + config.Loc.String() + ")"}, config.IdSuffixes...)
	case SynthTypes.UNLOCK:
		//config.IdSuffixes = append([]string{"mu-unlock(" + config.Loc.String() + ")"}, config.IdSuffixes...)
	case SynthTypes.RWMU_RLOCK:
		//config.IdSuffixes = append([]string{"rwmu-rlock(" + config.Loc.String() + ")"}, config.IdSuffixes...)
	case SynthTypes.RWMU_RUNLOCK:
		//config.IdSuffixes = append([]string{"rwmu-runlock(" + config.Loc.String() + ")"}, config.IdSuffixes...)
	case SynthTypes.WAITING:
		config.IdSuffixes = append([]string{"waiting"}, config.IdSuffixes...)
	case SynthTypes.WAKING:
		config.IdSuffixes = append([]string{"waking"}, config.IdSuffixes...)
	case SynthTypes.COND_WAIT:
		//config.IdSuffixes = append([]string{config.Loc.String() + ".Wait()"}, config.IdSuffixes...)
	case SynthTypes.COND_WAITING:
		//config.IdSuffixes = append([]string{config.Loc.String() + ".Waiting()"}, config.IdSuffixes...)
	case SynthTypes.COND_WAKING:
		//config.IdSuffixes = append([]string{config.Loc.String() + ".Waking()"}, config.IdSuffixes...)
	case SynthTypes.COND_SIGNAL:
		//config.IdSuffixes = append([]string{config.Loc.String() + ".Signal()"}, config.IdSuffixes...)
	case SynthTypes.COND_BROADCAST:
		//config.IdSuffixes = append([]string{config.Loc.String() + ".Broadcast()"}, config.IdSuffixes...)
	case SynthTypes.API_CONC_BUILTIN:
		config.IdSuffixes = append([]string{"api-builtin"}, config.IdSuffixes...)
	default:
		log.Fatal("Inexhaustive pattern match: ", config.Type)
		os.Exit(1)
	}

	// Compute a synthetic ID based on a string path of
	// the SSA node, plus suffixes. Works for SSA functions and instructions.
	nodeSyntheticId := func(node ssa.Node) string {

		suffix := strings.Join(config.IdSuffixes, "-")

		getFunStr := utils.SSAFunString
		getBlkStr := utils.SSABlockString
		if node != nil {
			switch n := node.(type) {
			case ssa.Instruction:
				return getBlkStr(n.Block()) + ":" + suffix
			case *ssa.Function:
				return getFunStr(n) + ":" + suffix
			}
		}
		return suffix
	}

	// Compute a synthetic ID based on a string path of
	// the SSA block, plus suffixes.
	blkSyntheticId := func(blk *ssa.BasicBlock) string {
		if len(blk.Instrs) == 0 {
			fmt.Println("Block does not have any instructions")
			os.Exit(1)
		}
		i := blk.Instrs[0].(ssa.Node)
		// Leverages ID construction to nodeSyntheticId,
		// using the first instruction in the block as an SSA node.
		return nodeSyntheticId(i)
	}

	// Determine which member of config to use for synthetic ID construction.
	// Provided instructions have the highest priority, followed by blocks, and
	// then functions. If none are present, only the type and provided suffixes are used.
	switch {
	case config.Insn != nil:
		return nodeSyntheticId(config.Insn.(ssa.Node))
	case config.Block != nil:
		return blkSyntheticId(config.Block)
	case config.Function != nil:
		return nodeSyntheticId(config.Function)
	default:
		return strings.Join(config.IdSuffixes, "-")
	}
}

// Initialize all structures required for a synthetic node,
// (maps for successors/predecessors/spawns), and set the
// parent block and function.
func (n *Synthetic) Init(config SynthConfig, id string) {
	n.succ = make(map[Node]struct{})
	n.pred = make(map[Node]struct{})
	n.spawn = make(map[Node]struct{})
	n.panickers = make(map[Node]struct{})
	n.id = id
	if config.Insn != nil {
		n.fun = config.Insn.Parent()
		n.block = config.Insn.Block()
	}
	if config.Block != nil {
		n.fun = config.Block.Parent()
		n.block = config.Block
	}
	if config.Function != nil {
		n.fun = config.Function
	}
}

// Function returns the parent function of a synthetic node.
func (n *Synthetic) Function() *ssa.Function {
	return n.fun
}

// Channel is not defined for arbitrary synthetic nodes.
func (n *Synthetic) Channel() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup Channel of %s", n))
}

// Mutex is not defined for arbitrary synthetic nodes.
func (n *Synthetic) Mutex() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup Mutex of %s", n))
}

// RWMutex is not defined for arbitrary synthetic nodes.
func (n *Synthetic) RWMutex() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup RWMutex of %s", n))
}

// Locker is not defined for arbitrary synthetic nodes.
func (n *Synthetic) Locker() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup Locker of %s", n))
}

// Cond is not defined for arbitrary synthetic nodes.
func (n *Synthetic) Cond() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup Cond of %s", n))
}

// WaitGroup is not defined for arbitrary synthetic nodes.
func (n *Synthetic) WaitGroup() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup WaitGroup of %s", n))
}

// Block returns the parent basic block of the synthetic node.
func (n *Synthetic) Block() *ssa.BasicBlock {
	return n.block
}

func (n *Select) Ops() []ChnSynthetic {
	return n.ops
}

func (n *Select) Channel() (ret ssa.Value) {
	// Allow getting the channel of a select statement
	// if it only has a single case with a channel operation
	// on it. Useful for mapping the channel value from
	// runtime.chanrecv/send with blocking=false back
	// to an ssa value.
	for _, op := range n.ops {
		if ch := op.Channel(); ch != nil {
			if ret != nil {
				panic("Called Channel() on a select statement with multiple communication branches")
			}

			ret = ch
		}
	}

	if ret == nil {
		panic("Called Channel() on a select statement without any communication branches")
	}

	return
}

// IsChannelOp is true for select nodes.
func (n *Select) IsChannelOp() bool {
	return true
}

// Cause returns the termination cause of a goroutine termination node.
func (n *TerminateGoro) Cause() _TERMINATION_CAUSE {
	return n.cause
}

// Args returns the arguments of a call to a built-in function.
func (n *BuiltinCall) Args() []ssa.Value {
	if n.Call != nil {
		return n.Call.Common().Args
	}
	return nil
}

// Arg retrieves the argument at the given position for a built-in call.
func (n *BuiltinCall) Arg(i int) ssa.Value {
	return n.Call.Common().Args[i]
}

// Builtin retrieves the underlying builtin for a node denoting a call to a builtin function.
func (n *BuiltinCall) Builtin() *ssa.Builtin {
	return n.builtin
}

// Channel returns the underlying channel operated on by a builtin call. For `close` and `len`
// builtins, it returns the first argument. It will panic for builtins not operating on channels.
func (n *BuiltinCall) Channel() ssa.Value {
	switch n.builtin.Name() {
	case "close":
		return n.Arg(0)
	case "len":
		// The built-in `len` only has a channel when the underlying type of
		// its argument is a channel.
		if len(n.Args()) != 1 {
			break
		}
		if _, ok := n.Arg(0).Type().Underlying().(*types.Chan); ok {
			return n.Arg(0)
		}
	}

	return n.Synthetic.Channel()
}

// IsCommunicationNode is false for arbitrary synthetic nodes.
func (n *Synthetic) IsCommunicationNode() bool {
	return false
}

// IsCommunicationNode checks whether a deferred function call is a concurrency operation.
func (n *DeferCall) IsCommunicationNode() bool {
	if dfr := n.dfr; dfr != nil {
		dfr, ok := dfr.(*SSANode)
		if !ok {
			return false
		}

		switch call := dfr.Instruction().(type) {
		case *ssa.Defer:
			return callCommonIsConcurrent(call.Call)
		}
		return false
	}
	return false
}

// Instruction returns the SSA instruction denoted by a deferred call.
func (n *DeferCall) Instruction() ssa.Instruction {
	if dfr := n.dfr; dfr != nil {
		dfr, ok := dfr.(*SSANode)
		if !ok {
			return nil
		}

		return dfr.Instruction()
	}
	return nil
}

// Channel is nil for a deferred call.
func (n *DeferCall) Channel() ssa.Value {
	return nil
}

// Cond returns the conditional value operated on by a deferred call or method invocation.
func (n *DeferCall) Cond() ssa.Value {
	if dfr := n.dfr; dfr != nil {
		if dfr, ok := dfr.(*SSANode); ok {
			return getCond(dfr.Instruction())
		}
	}
	return nil
}

// WaitGroup returns the waitgroup value operated on by a deferred call or method invocation.
func (n *DeferCall) WaitGroup() ssa.Value {
	if dfr := n.dfr; dfr != nil {
		if dfr, ok := dfr.(*SSANode); ok {
			return getWaitGroup(dfr.Instruction())
		}
	}
	return nil
}

// Mutex returns the mutex value operated on by a deferred call or method invocation.
func (n *DeferCall) Mutex() ssa.Value {
	if dfr := n.dfr; dfr != nil {
		dfr, ok := dfr.(*SSANode)
		if !ok {
			return nil
		}

		return getMutex(dfr.Instruction())
	}
	return nil
}

// RWMutex returns the read-write mutex value operated on by a deferred call or method invocation.
func (n *DeferCall) RWMutex() ssa.Value {
	if dfr := n.dfr; dfr != nil {
		dfr, ok := dfr.(*SSANode)
		if !ok {
			return nil
		}

		return getRWMutex(dfr.Instruction())
	}
	return nil
}

// Locker retrieves the locker operand of a given CF-node. May panic if the node is not locker-related.
func (n *DeferCall) Locker() ssa.Value {
	if dfr := n.dfr; dfr != nil {
		dfr, ok := dfr.(*SSANode)
		if !ok {
			return nil
		}

		return getLocker(dfr.Instruction())
	}

	return nil
}

// Locker retrieves the locker operand of a given CF-node. May panic if the node is not locker-related.
func (n *APIConcBuiltinCall) Locker() ssa.Value {
	return getLocker(n.Call)
}

// Cond retrieves the channel operand of a given CF-node. May panic if the node is not conditional variable-related.
func (n *APIConcBuiltinCall) Cond() ssa.Value {
	return getCond(n.Call)
}

// IsCommunicationNode is true for select operations.
func (n *Select) IsCommunicationNode() bool {
	return true
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *Select) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

// IsCommunicatioNode is true for receive cases of select statements.
func (n *SelectRcv) IsCommunicationNode() bool {
	return true
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *SelectRcv) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

// IsCommunicatioNode is true for send cases of select statements.
func (n *SelectSend) IsCommunicationNode() bool {
	return true
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *SelectSend) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

// IsCommunicatioNode is true for default cases of select statements.
func (n *SelectDefault) IsCommunicationNode() bool {
	return true
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *SelectDefault) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

// IsCommunicatioNode is true for nodes encoding goroutine termination.
func (n *TerminateGoro) IsCommunicationNode() bool {
	return true
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *TerminateGoro) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

// IsCommunicationNode checks whether the CF-node denotes a concurrency operation.
func (n *PendingGo) IsCommunicationNode() bool {
	return true
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *PendingGo) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

// IsCommunicatioNode is true for builtin call nodes where the builtin function is `close`
// or `len` on a channel.
func (n *BuiltinCall) IsCommunicationNode() bool {
	switch n.builtin.Name() {
	case "close":
		return true
	case "len":
		if n.Call.Value() == nil {
			// The builtin call to `len` is not relevant for concurrency when used with `defer` and `go`
			// because its result cannot be used elsewhere. Any interleavings can be discarded.
			break
		}

		if len(n.Args()) == 1 {
			// The builtin call to `len` is only relevant if its argument is a channel.
			_, ok := n.Arg(0).Type().Underlying().(*types.Chan)
			return ok
		}
	}
	return false
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *BuiltinCall) CommTransitive() map[Node]struct{} {
	if n.IsCommunicationNode() {
		return map[Node]struct{}{n: {}}
	}

	return n.Synthetic.CommTransitive()
}

// IsCommunicatioNode is true for nodes representing that a goroutine is waiting on a conditional variable.
func (n *Waiting) IsCommunicationNode() bool {
	return true
}

// IsCommunicatioNode is true for nodes representing that a goroutine is waking after waiting on a conditional variable.
func (n *Waking) IsCommunicationNode() bool {
	return true
}

// IsCommunicatioNode is true for invocations of methods on standard library concurrency primitives.
func (n *APIConcBuiltinCall) IsCommunicationNode() bool {
	return true
}

// Cond retrieves the conditional varaible operand of a given CF-node. May panic if the node is not conditional variable-related.
func (n *Waiting) Cond() ssa.Value {
	return n.Call.Common().Args[0]
}

// Cond retrieves the conditional varaible operand of a given CF-node. May panic if the node is not conditional variable-related.
func (n *Waking) Cond() ssa.Value {
	return n.Call.Common().Args[0]
}

// Instruction retrieves the SSA instruction underlying the CF-node.
func (n *Waiting) Instruction() ssa.Instruction {
	switch n := n.Predecessor().(type) {
	case *SSANode:
		return n.Instruction()
	case *DeferCall:
		return n.Instruction()
	}
	return nil
}

// Instruction retrieves the SSA instruction underlying the CF-node.
func (n *Waking) Instruction() ssa.Instruction {
	switch n := n.Predecessor().(type) {
	case *SSANode:
		return n.Instruction()
	case *DeferCall:
		return n.Instruction()
	}
	return nil
}

// Instruction retrieves the SSA instruction underlying the CF-node.
func (n *APIConcBuiltinCall) Instruction() ssa.Instruction {
	return n.Call
}

// CallInstruction retrieves the SSA call instruction underlying the CF-node.
func (n *Waiting) CallInstruction() ssa.CallInstruction {
	return n.Call
}

// CallInstruction retrieves the SSA call instruction underlying the CF-node.
func (n *Waking) CallInstruction() ssa.CallInstruction {
	return n.Call
}

// CallInstruction retrieves the SSA call instruction underlying the CF-node.
func (n *APIConcBuiltinCall) CallInstruction() ssa.CallInstruction {
	return n.Call
}

// Pos retrieves the source location position of the CF-node.
func (n *Waiting) Pos() token.Pos {
	return n.Call.Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *Waking) Pos() token.Pos {
	return n.Call.Pos()
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *Waiting) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *Waking) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *APIConcBuiltinCall) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *Synthetic) String() string {
	return fmt.Sprintf("[ %s ]", n.Id())
}

// Pos retrieves the source location position of the CF-node.
func (Synthetic) Pos() token.Pos {
	return token.NoPos
}

// CommTransitive retrieves the nearest communication transitive successors of current node.
// It returns the current node, if it represents a concurrency operation.
func (n *Synthetic) CommTransitive() map[Node]struct{} {
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

// Pos retrieves the source location position of the CF-node.
func (n *PostCall) Pos() token.Pos {
	return n.CallRelationNode().Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *DeferCall) Pos() token.Pos {
	return n.DeferLink().Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *PostDeferCall) Pos() token.Pos {
	return n.CallRelationNode().Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *FunctionEntry) Pos() token.Pos {
	return n.fun.Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *FunctionExit) Pos() token.Pos {
	return n.fun.Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *TerminateGoro) Pos() token.Pos {
	return n.fun.Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *SelectRcv) Pos() token.Pos {
	return n.pos
}

// Pos retrieves the source location position of the CF-node.
func (n *SelectSend) Pos() token.Pos {
	return n.pos
}

// Pos retrieves the source location position of the CF-node.
func (n *SelectDefault) Pos() token.Pos {
	return n.pos
}

// Pos retrieves the source location position of the CF-node.
func (n *Select) Pos() token.Pos {
	return n.Insn.Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *SelectDefer) Pos() token.Pos {
	return n.dfr.Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *BuiltinCall) Pos() token.Pos {
	return n.Call.Pos()
}

// Pos retrieves the source location position of the CF-node.
func (n *APIConcBuiltinCall) Pos() token.Pos {
	return n.Call.Pos()
}

func (n *BuiltinCall) String() string {
	callCommon := n.Call.Common()
	strs := make([]string, 0, len(callCommon.Args))
	for _, arg := range callCommon.Args {
		strs = append(strs, arg.Name())
	}

	var name string
	if call, ok := n.Call.(ssa.Value); ok {
		name = call.Name() + " = "
	}

	return "[ " + name + n.builtin.Name() + "(" + strings.Join(strs, ", ") + ") ]"
}

func (n *Select) String() string {
	cases := []string{}
	for _, op := range n.ops {
		opstr := op.String()
		cases = append(cases, opstr[2:len(opstr)-2])
	}
	if len(cases) == 0 {
		return "[ select ]"
	}
	return "[ select { " + strings.Join(cases, "; ") + " } ]"
}

func (n *SelectDefer) String() string {
	return "[ select-defer ]"
}

func (n *SelectDefault) String() string {
	return "[ default ]"
}

func (n *SelectRcv) String() string {
	str := "[ case "
	switch {
	case n.Ok != nil:
		str += n.Val.Name() + ", " + n.Ok.Name() + " = "
	case n.Val != nil:
		str += n.Val.Name() + " = "
	}

	return str + "<-" + n.chn.Name() + " ]"
}

func (n *SelectSend) String() string {
	return "[ case " + n.chn.Name() + " <- " + n.Val.String() + " ]"
}

func (n *TerminateGoro) String() string {
	switch n.cause {
	case GoroTermination.EXIT_ROOT:
		return "[ \u22A5 ]"
	case GoroTermination.PERMANENTLY_BLOCKED:
		return "[ ⛔ ]"
	case GoroTermination.INFINITE_LOOP:
		return "[ \u21B7 ]"
	default:
		log.Fatal("Unrecognized goroutine exit cause")
		os.Exit(1)
		return ""
	}
}

func (PendingGo) String() string {
	return "[ \u231B ]"
}

func (p PostCall) String() string {
	if p.CallRelationNode() == nil {
		return "[ post-call ]"
	}
	insn := p.CallRelationNode().SSANode().Instruction().(ssa.CallInstruction)
	return "[ post-call:" + insn.Value().Name() + " = " + insn.Value().String() + " ]"
}

func (n *DeferCall) String() string {
	var dfrStr string
	if n.dfr != nil {
		dfr1, ok := n.dfr.(*SSANode)
		if !ok {
			panic("what?")
		}
		dfr, ok := dfr1.Instruction().(*ssa.Defer)
		if !ok {
			panic("what?")
		}

		dfrStr = ": " + dfr.Call.String()
	}
	return "[ defer" + dfrStr + " ]"
}

func (PostDeferCall) String() string {
	return "[ post-defer ]"
}

func (n *FunctionEntry) String() string {
	return "[ " + n.Function().Name() + ":entry ]"
}

func (n *FunctionExit) String() string {
	return "[ " + n.Function().Name() + ":exit ]"
}

func (n *APIConcBuiltinCall) String() string {
	return "[ " + n.Call.String() + " ]"
}

func (n *Waiting) String() string {
	return "[ " + n.Cond().Name() + ".Waiting ]"
}

func (n *Waking) String() string {
	return "[ " + n.Cond().Name() + ".Waking ]"
}
