package lattice

//go:generate go run generate-map.go OpOutcomes string AbstractValue finite

const (
	_OUTCOME_OK    = "Succeed"
	_OUTCOME_PANIC = "Panic"
)

var outcomesLattice = &OpOutcomesLattice{
	mp: *MakeMapLatticeVariadic[string](valueLattice,
		_OUTCOME_OK,
		_OUTCOME_PANIC,
	),
}

func (latticeFactory) OpOutcomes() *OpOutcomesLattice {
	return outcomesLattice
}

func (l *OpOutcomesLattice) String() string {
	return colorize.Lattice("OperationOutcomes")
}

func (elementFactory) OpOutcomes() OpOutcomes {
	return outcomesLattice.Bot().OpOutcomes()
}

func (e OpOutcomes) MayPanic() bool {
	return !e.Get(_OUTCOME_PANIC).IsBot()
}

func (e OpOutcomes) MaySucceed() bool {
	return !e.Get(_OUTCOME_OK).IsBot()
}

func (e OpOutcomes) Blocks() bool {
	return e.mp.ForAll(func(key string, e Element) bool {
		return e.AbstractValue().IsBot()
	})
}

func (e OpOutcomes) OnPanic(do func(AbstractValue)) OpOutcomes {
	if val := e.Get(_OUTCOME_PANIC); !val.IsBot() {
		do(val)
	}
	return e
}

// Perform a procedure if there is a success outcome
func (e OpOutcomes) OnSucceed(do func(AbstractValue)) OpOutcomes {
	if val := e.Get(_OUTCOME_OK); !val.IsBot() {
		do(val)
	}
	return e
}
