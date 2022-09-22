package lattice

import (
	"go/token"
	"go/types"
	"testing"

	"github.com/cs-au-dk/goat/analysis/defs"
	loc "github.com/cs-au-dk/goat/analysis/location"

	"golang.org/x/tools/go/ssa"
)

type fakeValue struct{}

func (fakeValue) Name() string                  { return "fakeValue" }
func (fakeValue) Parent() *ssa.Function         { return nil }
func (fakeValue) Pos() token.Pos                { return token.NoPos }
func (fakeValue) Referrers() *[]ssa.Instruction { return nil }
func (f fakeValue) String() string              { return f.Name() }
func (fakeValue) Type() types.Type {
	return types.NewPointer(types.Typ[types.Int])
}

type fakeGoro struct{ name string }

func (*fakeGoro) Hash() uint32                            { return 42 }
func (a *fakeGoro) Equal(b defs.Goro) bool                { return a == b }
func (*fakeGoro) WeakEqual(b defs.Goro) bool              { return false }
func (f *fakeGoro) String() string                        { return "fake-goro: " + f.name }
func (*fakeGoro) CtrLoc() defs.CtrLoc                     { return defs.CtrLoc{} }
func (*fakeGoro) Index() int                              { return 0 }
func (*fakeGoro) Parent() defs.Goro                       { return nil }
func (f *fakeGoro) Root() defs.Goro                       { return f }
func (*fakeGoro) Spawn(defs.CtrLoc) defs.Goro             { panic("") }
func (*fakeGoro) SpawnIndexed(defs.CtrLoc, int) defs.Goro { panic("") }
func (f *fakeGoro) SetIndex(int) defs.Goro                { return f }
func (*fakeGoro) IsRoot() bool                            { return true }
func (*fakeGoro) IsChildOf(defs.Goro) bool                { return false }
func (*fakeGoro) IsParentOf(defs.Goro) bool               { return false }
func (*fakeGoro) IsCircular() bool                        { return false }
func (f *fakeGoro) GetRadix() defs.Goro                   { return f }
func (*fakeGoro) Length() int                             { return 1 }

var _ defs.Goro = &fakeGoro{}

func TestPointsTo(t *testing.T) {
	// TODO: These tests are limited in that they do not explore combinations
	// of different kinds of locations. Also combinations where some locations
	// have a top representative and others do not are missing.
	var site ssa.Value = &fakeValue{}

	al := loc.AllocationSiteLocation{
		Goro:    &fakeGoro{"a"},
		Context: nil,
		Site:    site,
	}
	bl := loc.AllocationSiteLocation{
		Goro:    &fakeGoro{"b"},
		Context: nil,
		Site:    site,
	}

	// tl represents both al and bl
	tl, ok := representative(al)
	if !ok {
		t.Fatal("???")
	}

	as := elFact.PointsTo(al)
	bs := elFact.PointsTo(bl)
	abs := elFact.PointsTo(al, bl)
	ts := elFact.PointsTo(tl)
	abts := elFact.PointsTo(al, bl, tl)
	emptyset := elFact.PointsTo()

	t.Run("Comparison", func(t *testing.T) {
		tests := []struct {
			a, b     PointsTo
			tf       func(Element) bool
			kind     string
			expected bool
		}{
			{as, as, as.leq, "⊑", true},
			{as, as, as.geq, "⊒", true},
			{as, as, as.eq, "=", true},

			{as, bs, as.leq, "⊑", false},
			{as, bs, as.geq, "⊒", false},
			{as, bs, as.eq, "=", false},

			{as, emptyset, as.leq, "⊑", false},
			{as, emptyset, as.geq, "⊒", true},
			{as, emptyset, as.eq, "=", false},

			{as, abs, as.leq, "⊑", true},
			{as, abs, as.geq, "⊒", false},
			{as, abs, as.eq, "=", false},

			{as, ts, as.leq, "⊑", true},
			{as, ts, as.geq, "⊒", false},
			{as, ts, as.eq, "=", false},

			{abs, ts, abs.leq, "⊑", true},
			{abs, ts, abs.geq, "⊒", false},
			{abs, ts, abs.eq, "=", false},

			{abts, ts, abts.leq, "⊑", true},
			{abts, ts, abts.geq, "⊒", true},
			{abts, ts, abts.eq, "=", true},
		}

		for _, test := range tests {
			res := test.tf(test.b)
			if res != test.expected {
				t.Errorf("Expected %v %s %v = %v but was %v",
					test.a, test.kind, test.b, test.expected, res)
			}
		}
	})

	t.Run("Join", func(t *testing.T) {
		join, meet := "⊔", "⊓"
		tests := []struct {
			a, b, expected PointsTo
			kind           string
		}{
			{as, as, as, join},
			{as, bs, abs, join},

			{as, bs, emptyset, meet},

			{as, ts, ts, join},
			{abs, abts, ts, join},
			{abs, ts, abs, meet},
			{abts, ts, ts, meet},
		}

		for _, test := range tests {
			var res PointsTo
			if test.kind == join {
				res = test.a.MonoJoin(test.b)
			} else {
				res = test.a.MonoMeet(test.b)
			}

			if !res.eq(test.expected) {
				t.Errorf("Expected %v %s %v = %v but was %v",
					test.a, test.kind, test.b, test.expected, res)
			}
		}
	})
}
