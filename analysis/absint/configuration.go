package absint

import (
	L "Goat/analysis/lattice"
	"fmt"
	"strconv"
)

type ABSTRACTION_LEVEL int

func (a ABSTRACTION_LEVEL) String() string {
	switch a {
	case ABS_CONCRETE:
		return "Concrete level"
	case ABS_BUFFER:
		return "Buffer level - flat"
	case ABS_COARSE:
		return "Coarsest level"
	default:
		return "Unknown abstraction level " + strconv.Itoa((int)(a))
	}
}

const (
	ABS_CONCRETE = iota
	ABS_BUFFER
	ABS_COARSE
)

type ConfigurationSuccessors map[string]Successor

func (S ConfigurationSuccessors) PrettyPrint() {
	fmt.Println("Successor configurations:")
	for _, succ := range S {
		succ.PrettyPrint()
	}
}

type Configuration interface {
	AbstractionLevel() ABSTRACTION_LEVEL
	Init(ABSTRACTION_LEVEL) Configuration

	String() string
	PrettyPrint()

	IsSynchronizing(AnalysisCtxt, L.AnalysisState) bool

	AddSuccessor(Successor)
	GetSuccessorMap() map[uint32]Successor
	GetTransitions(AnalysisCtxt, L.AnalysisState) transfers
	Abstract(ABSTRACTION_LEVEL) Configuration
	Visualize()
}

type BaseConfiguration struct {
	Successors map[uint32]Successor
}

func (s *BaseConfiguration) AddSuccessor(succ Successor) {
	s.Successors[succ.Hash()] = succ
}

func (s *BaseConfiguration) GetSuccessorMap() map[uint32]Successor {
	return s.Successors
}

func NewConfiguration(abs ABSTRACTION_LEVEL) Configuration {
	var s Configuration
	switch abs {
	case ABS_COARSE:
		s = new(AbsConfiguration)
	}
	s.Init(abs)
	return s
}
