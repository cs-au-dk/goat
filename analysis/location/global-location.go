package location

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

// GlobalLocation represents the heap location of a global variable. The allocation
// site is guaranteed to be a global SSA register.
type GlobalLocation struct {
	addressable
	Site *ssa.Global
}

func (l GlobalLocation) Equal(ol Location) bool {
	o, ok := ol.(GlobalLocation)
	return ok && l == o
}

func (l GlobalLocation) Position() string {
	if l.Site.Pkg != nil {
		return l.Site.Pkg.Prog.Fset.Position(l.Site.Pos()).String()
	}

	return ""
}

func (l GlobalLocation) Hash() uint32 {
	return phasher.Hash(l.Site)
}

func (l GlobalLocation) String() string {
	return colorize.Cons("Global") + "(" +
		colorize.Site(l.Site.Name()) + ")"
}

func (l GlobalLocation) GetSite() (ssa.Value, bool) {
	return l.Site, l.Site != nil
}

func (l GlobalLocation) Type() types.Type {
	if l.Site == nil {
		return nil
	}
	return l.Site.Type()
}
