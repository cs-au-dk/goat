package location

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// NilLocation represents the nil pointer.
type NilLocation struct{}

func (n NilLocation) Hash() uint32 {
	return 42
}

func (n NilLocation) Equal(o Location) bool {
	_, ok := o.(NilLocation)
	return ok
}

func (n NilLocation) Position() string {
	return "<nil.Position>"
}

func (n NilLocation) String() string {
	return colorize.Nil("nil")
}

func (l NilLocation) GetSite() (ssa.Value, bool) {
	return nil, false
}

func (l NilLocation) Type() types.Type {
	return nil
}
