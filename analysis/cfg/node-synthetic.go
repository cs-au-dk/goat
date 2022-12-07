package cfg

import (
	"fmt"
	"go/token"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

type SYNTH_TYPE_ID = int

// Collection of IDs for synthetic node types.
// Wrapped in a struct to avoid excessively polluting the namespace.
var SynthTypes = struct {
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
	API_CONC_BUILTIN:  32,
}

// Basic synthetic node structure.
// All synthetic node types should embed it.
type Synthetic struct {
	BaseNode
	fun   *ssa.Function
	block *ssa.BasicBlock
	id    string
}

type BlockEntry struct{ Synthetic }
type BlockExit struct{ Synthetic }
type BlockEntryDefer struct{ Synthetic }
type BlockExitDefer struct{ Synthetic }
type FunctionEntry struct{ Synthetic }
type FunctionExit struct{ Synthetic }
type PendingGo struct{ Synthetic }
type TerminateGoro struct {
	Synthetic
	cause _TERMINATION_CAUSE
}

type _TERMINATION_CAUSE = int

var GoroTermination = struct {
	EXIT_ROOT           _TERMINATION_CAUSE
	PERMANENTLY_BLOCKED _TERMINATION_CAUSE
	INFINITE_LOOP       _TERMINATION_CAUSE
}{
	EXIT_ROOT:           0,
	PERMANENTLY_BLOCKED: 1,
	INFINITE_LOOP:       2,
}

// Interface for synthetic nodes involving channel makechan SSA values.
type ChnSynthetic interface {
	AnySynthetic
	Channel() ssa.Value
}

// Structure underlying any ChnSynthetic.
type chnSynthetic struct {
	Synthetic
	chn ssa.Value
}

type selectChnSynthetic struct {
	chnSynthetic
	Parent *Select
	pos    token.Pos
}
type SelectSend struct {
	selectChnSynthetic
	Val ssa.Value
}
type SelectRcv struct {
	selectChnSynthetic
	Val ssa.Value
	Ok  ssa.Value
}
type SelectDefault struct {
	selectChnSynthetic
}
type Select struct {
	Synthetic
	ops  []ChnSynthetic
	Insn *ssa.Select
}
type SelectDefer struct{ Synthetic }
type DeferCall struct{ Synthetic }
type PostDeferCall struct{ Synthetic }
type PostCall struct{ Synthetic }
type BuiltinCall struct {
	Synthetic
	Call    ssa.CallInstruction
	builtin *ssa.Builtin
}
type Waiting struct {
	Synthetic
	Call ssa.CallInstruction
}
type Waking struct {
	Synthetic
	Call ssa.CallInstruction
}
type APIConcBuiltinCall struct {
	Synthetic
	Call ssa.CallInstruction
}

func (n *chnSynthetic) Channel() ssa.Value {
	return n.chn
}

// Universal implementation of synthetic node configuration.
// Used when creating synthetic nodes. When instantiating,
// only the necessary fields need to be specified,
// depending on the type of the defined node.
type SynthConfig struct {
	// Mandatory: specifies the type of synthetic node.
	Type SYNTH_TYPE_ID
	// Parent function
	Function *ssa.Function
	// Parent basic block
	Block *ssa.BasicBlock
	// Associated instruction
	Insn ssa.Instruction
	// Used for nodes related to channel values
	Chn *ssa.MakeChan
	// Other locations which might be used
	Call ssa.CallInstruction
	// Arbitrary list of values.
	// Different uses for different types of synthetic nodes
	// -- Builtin calls:
	//			0 - builtin function
	// -- Select send case nodes:
	//			0 - channel
	//			1 - sent value
	// -- Select receive case nodes:
	//			0 - channel
	//			1 - received value (if receive with assignment)
	//			2 - received ok (if receive with tuple assignment)
	Vals []ssa.Value
	// The index of case branch in a select statement
	SelectIndex int
	// All case branches of a synthetic select node
	SelectOps []ChnSynthetic
	// The parent select node of a select case branch node
	SelectParent *Select
	// The source position of the select branch
	Pos token.Pos
	// Cause of goroutine termination
	TerminationCause _TERMINATION_CAUSE
	// Optional suffixes
	IdSuffixes []string
}

// All synthetic nodes must implement this interface
type AnySynthetic interface {
	// Extends the universal Node interface
	Node
	// Synthetic node ID
	Id() string
	// Parent basic block
	Block() *ssa.BasicBlock
	// Initialization synthetic node structures,
	// e. g. successor/predecessor maps
	Init(config SynthConfig, id string)
}

func (n *Synthetic) Id() string {
	return n.id
}

// Public API for creating synthetic nodes.
// Useful for creating stand-alone nodes
// outside the globally computed CFG
func CreateSynthetic(config SynthConfig) (n Node) {
	// Create synthetic ID here (avoids recomputing later).
	return createSynthetic(config, syntheticId(config))
}

// Public wrapper around addSynthetic. Like `CreateSynthetic` but does not
// generate a new node if a identical previous one exists.
func (cfg *Cfg) AddSynthetic(config SynthConfig) Node {
	node, _ := cfg.addSynthetic(config)
	return node
}

// Create a synthetic node, given the configuration and computed ID.
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
	n.spawners = make(map[Node]struct{})
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

func (n *Synthetic) Function() *ssa.Function {
	return n.fun
}

func (n *Synthetic) Channel() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup Channel of %s", n))
}

func (n *Synthetic) Mutex() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup Mutex of %s", n))
}

func (n *Synthetic) RWMutex() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup RWMutex of %s", n))
}

func (n *Synthetic) Locker() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup Locker of %s", n))
}

func (n *Synthetic) Cond() ssa.Value {
	panic(fmt.Sprintf("Cannot lookup Cond of %s", n))
}

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

func (n *Select) IsChannelOp() bool {
	return true
}

func (n *TerminateGoro) Cause() _TERMINATION_CAUSE {
	return n.cause
}

func (n *BuiltinCall) Args() []ssa.Value {
	if n.Call != nil {
		return n.Call.Common().Args
	}
	return nil
}

func (n *BuiltinCall) Arg(i int) ssa.Value {
	return n.Call.Common().Args[i]
}

func (n *BuiltinCall) Builtin() *ssa.Builtin {
	return n.builtin
}

func (n *BuiltinCall) Channel() ssa.Value {
	if n.builtin.Name() == "close" {
		return n.Arg(0)
	}

	return n.Synthetic.Channel()
}

func (n *Synthetic) IsCommunicationNode() bool {
	return false
}

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

func (n *DeferCall) Channel() ssa.Value {
	return nil
}

func (n *DeferCall) Cond() ssa.Value {
	if dfr := n.dfr; dfr != nil {
		if dfr, ok := dfr.(*SSANode); ok {
			return getCond(dfr.Instruction())
		}
	}
	return nil
}

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

func (n *APIConcBuiltinCall) Locker() ssa.Value {
	return getLocker(n.Call)
}

func (n *APIConcBuiltinCall) Cond() ssa.Value {
	return getCond(n.Call)
}

func (n *Select) IsCommunicationNode() bool {
	return true
}

func (n *Select) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *SelectRcv) IsCommunicationNode() bool {
	return true
}

func (n *SelectRcv) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *SelectSend) IsCommunicationNode() bool {
	return true
}

func (n *SelectSend) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *SelectDefault) IsCommunicationNode() bool {
	return true
}

func (n *SelectDefault) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *TerminateGoro) IsCommunicationNode() bool {
	return true
}

func (n *TerminateGoro) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *PendingGo) IsCommunicationNode() bool {
	return true
}

func (n *PendingGo) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *BuiltinCall) IsCommunicationNode() bool {
	return n.builtin.Name() == "close"
}

func (n *BuiltinCall) CommTransitive() map[Node]struct{} {
	if n.builtin.Name() == "close" {
		return map[Node]struct{}{n: {}}
	}

	return n.Synthetic.CommTransitive()
}

func (n *Waiting) IsCommunicationNode() bool {
	return true
}

func (n *Waking) IsCommunicationNode() bool {
	return true
}

func (n *APIConcBuiltinCall) IsCommunicationNode() bool {
	return true
}

func (n *Waiting) Cond() ssa.Value {
	return n.Call.Common().Args[0]
}

func (n *Waking) Cond() ssa.Value {
	return n.Call.Common().Args[0]
}

func (n *Waiting) Instruction() ssa.Instruction {
	switch n := n.Predecessor().(type) {
	case *SSANode:
		return n.Instruction()
	case *DeferCall:
		return n.Instruction()
	}
	return nil
}

func (n *Waking) Instruction() ssa.Instruction {
	switch n := n.Predecessor().(type) {
	case *SSANode:
		return n.Instruction()
	case *DeferCall:
		return n.Instruction()
	}
	return nil
}

func (n *APIConcBuiltinCall) Instruction() ssa.Instruction {
	return n.Call
}

func (n *Waiting) CallInstruction() ssa.CallInstruction {
	return n.Call
}

func (n *Waking) CallInstruction() ssa.CallInstruction {
	return n.Call
}

func (n *APIConcBuiltinCall) CallInstruction() ssa.CallInstruction {
	return n.Call
}

func (n *Waiting) String() string {
	return "[ " + n.Cond().Name() + ".Waiting ]"
}

func (n *Waking) String() string {
	return "[ " + n.Cond().Name() + ".Waking ]"
}

func (n *Waiting) Pos() token.Pos {
	return n.Call.Pos()
}

func (n *Waking) Pos() token.Pos {
	return n.Call.Pos()
}

func (n *Waiting) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *Waking) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *APIConcBuiltinCall) CommTransitive() map[Node]struct{} {
	return map[Node]struct{}{n: {}}
}

func (n *Synthetic) String() string {
	return fmt.Sprintf("[ %s ]", n.Id())
}

func (n *SelectSend) Payload() ssa.Value {
	return n.Val
}

func (n *SelectRcv) CommaOk() bool {
	return n.Ok != nil
}

func (Synthetic) Pos() token.Pos {
	return token.NoPos
}

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

func (n *PostCall) Pos() token.Pos {
	return n.CallRelationNode().Pos()
}

func (n *DeferCall) Pos() token.Pos {
	return n.DeferLink().Pos()
}

func (n *PostDeferCall) Pos() token.Pos {
	return n.CallRelationNode().Pos()
}

func (n *FunctionEntry) Pos() token.Pos {
	return n.fun.Pos()
}

func (n *FunctionExit) Pos() token.Pos {
	return n.fun.Pos()
}

func (n *TerminateGoro) Pos() token.Pos {
	return n.fun.Pos()
}

func (n *SelectRcv) Pos() token.Pos {
	return n.pos
}

func (n *SelectSend) Pos() token.Pos {
	return n.pos
}

func (n *SelectDefault) Pos() token.Pos {
	return n.pos
}

func (n *Select) Pos() token.Pos {
	return n.Insn.Pos()
}

func (n *SelectDefer) Pos() token.Pos {
	return n.dfr.Pos()
}

func (n *BuiltinCall) Pos() token.Pos {
	return n.Call.Pos()
}

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
