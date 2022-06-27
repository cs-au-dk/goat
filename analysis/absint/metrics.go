package absint

import (
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	"Goat/pkgutil"
	"fmt"
	"time"

	"golang.org/x/tools/go/ssa"
)

var (
	OUTCOME_BUGS_FOUND    = "Bugs found"
	OUTCOME_NO_BUGS_FOUND = "No bugs found"
	OUTCOME_PANIC         = "Panicked"
	OUTCOME_SKIP          = "Skipped"
)

type Metrics struct {
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

func (p prepAI) InitializeMetrics() func(*ssa.Function) *Metrics {
	if p.metrics {
		return initMetrics
	}
	return func(_ *ssa.Function) *Metrics {
		return nil
	}
}

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

func (m *Metrics) Enabled() bool {
	return m != nil
}

func (m *Metrics) ConcurrencyOps() map[ssa.Instruction]struct{} {
	if m == nil {
		return nil
	}
	return m.concurrencyOps
}

func (m *Metrics) Gos() map[ssa.Instruction]struct{} {
	if m == nil {
		return nil
	}
	return m.gos
}

func (m *Metrics) Chans() map[ssa.Instruction]struct{} {
	if m == nil {
		return nil
	}
	return m.chans
}

func (m *Metrics) HasConcurrency() bool {
	if m == nil {
		return false
	}

	return len(m.concurrencyOps) > 0
}

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

func (m *Metrics) TimerStart() {
	if m == nil {
		return
	}

	m.timer = time.Now()
}

func (m *Metrics) timerStop() {
	if m == nil {
		return
	}

	m.time = time.Since(m.timer)
}

func (m *Metrics) Functions() map[*ssa.Function]int {
	if m == nil {
		return nil
	}

	return m.expandedFunctions
}

func (m *Metrics) Performance() string {
	if m == nil {
		return "- no metrics gathered -"
	}

	return m.time.String()
}

func (m *Metrics) Skip() {
	if m == nil || m.Outcome != "" {
		return
	}

	m.Outcome = OUTCOME_SKIP
	m.timerStop()
	close(m.skipped)
}

func (m *Metrics) Panic(err interface{}) {
	if m == nil || m.Outcome != "" {
		return
	}

	m.Outcome = OUTCOME_PANIC
	m.timerStop()
	m.errorMsg = err
	close(m.skipped)
}

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
func (m *Metrics) Blocks() Blocks {
	if m == nil {
		return nil
	}

	return m.blocks
}

func (m *Metrics) SetBlocks(blocks Blocks) {
	if m == nil {
		return
	}

	m.blocks = blocks
}

func (m *Metrics) IsRelevant() bool {
	if m == nil {
		return false
	}
	return m.HasConcurrency() ||
		m.Outcome == OUTCOME_PANIC
}

func (m *Metrics) Error() string {
	if m == nil {
		return ""
	}

	return fmt.Sprint(m.errorMsg)
}

type funset = map[*ssa.Function]struct{}

func (m *Metrics) AddCallees(callIns ssa.CallInstruction, calleeSet funset) {
	for callee := range calleeSet {
		if _, ok := m.callsiteCallees[callIns]; !ok {
			m.callsiteCallees[callIns] = make(funset)
		}
		m.callsiteCallees[callIns][callee] = struct{}{}
	}
}

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

func (m *Metrics) CallsiteCallees() map[ssa.CallInstruction]funset {
	return m.callsiteCallees
}
