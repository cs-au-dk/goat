package location

import (
	"fmt"
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// Function pointer contains an *ssa.Function. Used for function values that do not need closures.
// See absint.evaluateSSA comment.
type FunctionPointer struct {
	Fun *ssa.Function
}

func (fp FunctionPointer) Hash() uint32 {
	return phasher.Hash(fp.Fun)
}
func (fp FunctionPointer) Position() string {
	return fp.Fun.Parent().Prog.Fset.Position(fp.Fun.Pos()).String()
}

func (fp FunctionPointer) Equal(ol Location) bool {
	o, ok := ol.(FunctionPointer)
	return ok && fp == o
}

func (fp FunctionPointer) String() string {
	return fmt.Sprintf("Function(%s)", fp.Fun)
}

func (l FunctionPointer) GetSite() (ssa.Value, bool) {
	return nil, false
}

func (l FunctionPointer) Type() types.Type {
	if l.Fun == nil {
		return nil
	}
	return l.Fun.Type()
}
