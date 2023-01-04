package lattice

//go:generate go run generate-map.go op-outcomes

// Operation outcomes are given as an enumeration, where outcomes may be either
// success or panic.
type outcomeName int

func (o outcomeName) String() string {
	switch o {
	case _OUTCOME_OK:
		return "Success"
	case _OUTCOME_PANIC:
		return "Panic"
	}
	return "!!INVALID OUTCOME!!"
}

const (
	_OUTCOME_INVALID outcomeName = iota
	_OUTCOME_OK
	_OUTCOME_PANIC
)

// outcomesLattice is a singleton instantiation of the lattice of operation outcomes.
var outcomesLattice = &OpOutcomesLattice{
	mp: *MakeMapLatticeVariadic[outcomeName](valueLattice,
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

// MayPanic checks whether the outcome of an operation may be a panic.
// An outcome O is considered as potentially panicking if ⊥ ⊏ O(Panic)
func (e OpOutcomes) MayPanic() bool {
	return !e.Get(_OUTCOME_PANIC).IsBot()
}

// MaySucceed checks whether the outcome of an operation may be successful without panicking.
// An outcome O is considered as potentially successful if ⊥ ⊏ O(Success)
func (e OpOutcomes) MaySucceed() bool {
	return !e.Get(_OUTCOME_OK).IsBot()
}

// Blocks checks whether the outcome of an operation may be blocking. An operation blocks
// if O(x) = ⊥, ∀ x ∈ dom(O).
func (e OpOutcomes) Blocks() bool {
	return e.mp.ForAll(func(key outcomeName, e Element) bool {
		return e.AbstractValue().IsBot()
	})
}

// OnPanic performs a procedure on the abstract value registered at the outcome of an operation,
// if panics are a potential outcome. The same outcome value is returned so that other methods may
// be chained.
func (e OpOutcomes) OnPanic(do func(AbstractValue)) OpOutcomes {
	if val := e.Get(_OUTCOME_PANIC); !val.IsBot() {
		do(val)
	}
	return e
}

// OnSucceed performs a procedure on the abstract value registered at the outcome of an operation,
// if success is a potential outcome. The same outcome value is returned so that other methods may
// be chained.
func (e OpOutcomes) OnSucceed(do func(AbstractValue)) OpOutcomes {
	if val := e.Get(_OUTCOME_OK); !val.IsBot() {
		do(val)
	}
	return e
}
