package location

import (
	"fmt"
	"go/types"

	"github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/utils"
	"golang.org/x/tools/go/ssa"
)

// AllocationSiteLocation encodes an abstract heap location created
// through a make/new instruction. Allocation sites are addressable,
// and are identified by the product between a context, goroutine ID and
// an SSA value representing the allocation instruction.
type AllocationSiteLocation struct {
	addressable
	Goro    utils.Hashable
	Context Context
	Site    ssa.Value
}

func (l AllocationSiteLocation) Equal(ol Location) bool {
	o, ok := ol.(AllocationSiteLocation)
	return ok && l == o
}

func (l AllocationSiteLocation) Position() string {
	if l.Site.Parent() != nil {
		return l.Site.Parent().Prog.Fset.Position(l.Site.Pos()).String()
	}

	return ""
}

func (l AllocationSiteLocation) Hash() uint32 {
	var ctxHash uint32
	if l.Context != nil {
		ctxHash = phasher.Hash(l.Context)
	}

	return utils.HashCombine(
		l.Goro.Hash(),
		ctxHash,
		phasher.Hash(l.Site),
	)
}

func (l AllocationSiteLocation) String() string {
	name := colorize.Site(l.Site.String())
	if realName, ok := upfront.ChannelNames[l.Site.Pos()]; ok {
		name = realName
	}

	var ctx string
	if l.Context != nil {
		ctx += " " + colorize.Context(l.Context)
	}

	var pos string

	return fmt.Sprintf("‹%s:%s %s%s›",
		l.Goro,
		ctx,
		name,
		pos)
}

// GetSite retrieves the SSA instruction performing the allocation.
func (l AllocationSiteLocation) GetSite() (ssa.Value, bool) {
	return l.Site, l.Site != nil
}

func (l AllocationSiteLocation) Type() types.Type {
	if l.Site == nil {
		return nil
	}
	return l.Site.Type()
}
