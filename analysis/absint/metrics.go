package absint

import (
	"fmt"
	"time"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/pkgutil"

	"golang.org/x/tools/go/ssa"
)

type (
	// Metrics encodes mechanisms for logging execution metrics.
	Metrics struct {
		callsiteCallees   map[ssa.CallInstruction]funset
		expandedFunctions map[*ssa.Function]int
		concurrencyOps    map[ssa.Instruction]struct{}
		chans             map[ssa.Instruction]struct{}
		gos               map[ssa.Instruction]struct{}
		time              time.Duration
		blocks            Blocks
		Outcome           string
		timer             time.Time
		skipped           chan struct{}
		errorMsg          interface{}
	}

	// funset is the type of sets of functions.
	funset = map[*ssa.Function]struct{}
)

// Encoding of metric outcomes.
var (
	OUTCOME_BUGS_FOUND    = "Bugs found"
	OUTCOME_NO_BUGS_FOUND = "No bugs found"
	OUTCOME_PANIC         = "Panicked"
	OUTCOME_SKIP          = "Skipped"
)

// InitializeMetrics returns a metrics initialization function that takes
// SSA functions as input and generates a Metrics object.
func (p AIConfig) InitializeMetrics() func(*ssa.Function) *Metrics {
	if p.Metrics {
		return initMetrics
	}
	return func(_ *ssa.Function) *Metrics {
		return nil
	}
}

// initMetrics initializes a Metrics object.
func initMetrics(entry *ssa.Function) *Metrics {
	return &Metrics{
		callsiteCallees: make(map[ssa.CallInstruction]map[*ssa.Function]struct{}),
		expandedFunctions: map[*ssa.Function]int{
			entry: 1,
		},
		concurrencyOps: make(map[ssa.Instruction]struct{}),
		chans:          make(map[ssa.Instruction]struct{}),
		gos:            make(map[ssa.Instruction]struct{}),
		skipped:        make(chan struct{}),
	}
}

// Enabled checks whether the Metrics object is available.
func (m *Metrics) Enabled() bool {
	return m != nil
}

// ConcurrencyOps returns the set of encountered concurrency operations.
func (m *Metrics) ConcurrencyOps() map[ssa.Instruction]struct{} {
	if m == nil {
		return nil
	}
	return m.concurrencyOps
}

// Gos returns the set of encountered `go` instructions.
func (m *Metrics) Gos() map[ssa.Instruction]struct{} {
	if m == nil {
		return nil
	}
	return m.gos
}

// Chans returns the set of encountered channels.
func (m *Metrics) Chans() map[ssa.Instruction]struct{} {
	if m == nil {
		return nil
	}
	return m.chans
}

// HasConcurrency checks whether more than one concurrency operation was encountered.
func (m *Metrics) HasConcurrency() bool {
	if m == nil {
		return false
	}

	return len(m.concurrencyOps) > 0
}

// AddCommOp registers that a communication operation was encountered.
func (m *Metrics) AddCommOp(cl defs.CtrLoc) {
	if m == nil || pkgutil.CheckInGoroot(cl.Node().Function()) {
		return
	}

	select {
	case <-m.skipped:
	default:
		switch n := cl.Node().(type) {
		case *cfg.SSANode:
			if n.IsCommunicationNode() {
				m.concurrencyOps[n.Instruction()] = struct{}{}
			}
		case *cfg.Select:
			m.concurrencyOps[n.Insn] = struct{}{}
		case *cfg.BuiltinCall:
			if n.IsCommunicationNode() {
				m.concurrencyOps[n.Call] = struct{}{}
			}
		case *cfg.DeferCall:
			if n.IsCommunicationNode() {
				m.concurrencyOps[n.Instruction()] = struct{}{}
			}
		}
	}
}

// AddGo registers that a `go` instruction was encoutnered by the analysis.
func (m *Metrics) AddGo(cl defs.CtrLoc) {
	if m == nil || pkgutil.CheckInGoroot(cl.Node().Function()) {
		return
	}

	select {
	case <-m.skipped:
	default:
		switch n := cl.Node().(type) {
		case *cfg.SSANode:
			if _, ok := n.Instruction().(*ssa.Go); ok {
				m.gos[n.Instruction()] = struct{}{}
			}
		}
	}
}

// AddChan registers that a channel allocation was encountered.
func (m *Metrics) AddChan(cl defs.CtrLoc) {
	if m == nil || pkgutil.CheckInGoroot(cl.Node().Function()) {
		return
	}

	select {
	case <-m.skipped:
	default:
		switch n := cl.Node().(type) {
		case *cfg.SSANode:
			if _, ok := n.Instruction().(*ssa.MakeChan); ok {
				m.chans[n.Instruction()] = struct{}{}
			}
		}
	}
}

// ExpandFunction registers that how often a function was expanded.
func (m *Metrics) ExpandFunction(f *ssa.Function) {
	if m == nil {
		return
	}

	select {
	case <-m.skipped:
	default:
		m.expandedFunctions[f]++
	}
}

// TimerStart starts a timer before the analysis runs.
func (m *Metrics) TimerStart() {
	if m == nil {
		return
	}

	m.timer = time.Now()
}

// timerStop stops the timer and registers the duration of the analysis.
func (m *Metrics) timerStop() {
	if m == nil {
		return
	}

	m.time = time.Since(m.timer)
}

// Functions returns the set of expanded functions.
func (m *Metrics) Functions() map[*ssa.Function]int {
	if m == nil {
		return nil
	}

	return m.expandedFunctions
}

// Performance logs how fast the analysis ran.
func (m *Metrics) Performance() string {
	if m == nil {
		return "- no metrics gathered -"
	}

	return m.time.String()
}

// Skip instructs to skip the current analysis.
func (m *Metrics) Skip() {
	if m == nil || m.Outcome != "" {
		return
	}

	m.Outcome = OUTCOME_SKIP
	m.timerStop()
	close(m.skipped)
}

// Panic instructs that an analysis threw an exception.
func (m *Metrics) Panic(err interface{}) {
	if m == nil || m.Outcome != "" {
		return
	}

	m.Outcome = OUTCOME_PANIC
	m.timerStop()
	m.errorMsg = err
	close(m.skipped)
}

// Done instructs that the analysis is done, and whether the analysis reported any bugs.
func (m *Metrics) Done() {
	if m == nil {
		return
	}

	m.timerStop()
	if len(m.blocks) > 0 {
		m.Outcome = OUTCOME_BUGS_FOUND
	} else {
		m.Outcome = OUTCOME_NO_BUGS_FOUND
	}
}

// Blocks returns the blocks detected by the analysis.
func (m *Metrics) Blocks() Blocks {
	if m == nil {
		return nil
	}

	return m.blocks
}

// SetBlocks is updates the discovered blocks.
func (m *Metrics) SetBlocks(blocks Blocks) {
	if m == nil {
		return
	}

	m.blocks = blocks
}

// IsRelevent checks that the analysis run was relevant, by having concurrency operations
// and not panicking.
func (m *Metrics) IsRelevant() bool {
	if m == nil {
		return false
	}
	return m.HasConcurrency() ||
		m.Outcome == OUTCOME_PANIC
}

// Error prints the error message resulting from running the analysis.
func (m *Metrics) Error() string {
	if m == nil {
		return ""
	}

	return fmt.Sprint(m.errorMsg)
}

// AddCallees expands the set of callees at a given call instruction.
func (m *Metrics) AddCallees(callIns ssa.CallInstruction, calleeSet funset) {
	for callee := range calleeSet {
		if _, ok := m.callsiteCallees[callIns]; !ok {
			m.callsiteCallees[callIns] = make(funset)
		}
		m.callsiteCallees[callIns][callee] = struct{}{}
	}
}

// MaxCallees collects the call instruction with the largest number of potential callees
// and the number of callees.
func (m *Metrics) MaxCallees() (ins ssa.CallInstruction, max int) {
	for i, callees := range m.callsiteCallees {
		calleeCount := len(callees)

		if max < len(callees) {
			max = calleeCount
			ins = i
		}
	}

	return
}

// CallsiteCallees returns the callees at a call instruction.
func (m *Metrics) CallsiteCallees() map[ssa.CallInstruction]funset {
	return m.callsiteCallees
}
