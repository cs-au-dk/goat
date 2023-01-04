package testutil

import (
	"strings"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"

	"golang.org/x/tools/go/expect"
	"golang.org/x/tools/go/ssa"
)

type Annotation interface {
	// Returns related annotations (created from notes on the same line).
	Related() annList
	String() string

	Panicked() bool

	Note() *expect.Note
	Nodes() map[cfg.Node]struct{}
	Manager() NotesManager
}

type AnnotationWithFocus interface {
	Annotation

	Focused() annList
}

type AnnPanicked struct {
	basicAnnotation
}

func (a AnnPanicked) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)
	return At(id_PANICKED + " at " + npos.String())
}

type AnnProgress interface {
	Annotation

	Focused() AnnGoro
	HasFocus() bool
}

type AnnBlocks struct {
	basicAnnotation
	goro    string
	goroann *AnnGoro
}

func (a AnnBlocks) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)
	var gostr string
	if a.goro != "" {
		gostr = a.Focused().String()
	}
	return At(gostr + " " + id_BLOCKS + " at " + npos.String())
}

func (a AnnBlocks) HasFocus() bool {
	return a.goro != ""
}

func (a AnnBlocks) Focused() AnnGoro {
	if a.goroann == nil {
		a.goroann = new(AnnGoro)
		*a.goroann = a.mgr.FindGoro(a.goro)
	}
	return *a.goroann
}

func (a AnnBlocks) Nodes() map[cfg.Node]struct{} {
	return commNodes(a)
}

type AnnReleases struct {
	basicAnnotation
	goro    string
	goroann *AnnGoro
}

func (a AnnReleases) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)
	return At(id_MAY_RELEASE + " at " + npos.String())
}

func (a AnnReleases) HasFocus() bool {
	return a.goro != ""
}

func (a AnnReleases) Focused() AnnGoro {
	if a.goroann == nil {
		a.goroann = new(AnnGoro)
		*a.goroann = a.mgr.FindGoro(a.goro)
	}
	return *a.goroann
}

func (a AnnReleases) Nodes() map[cfg.Node]struct{} {
	return commNodes(a.basicAnnotation)
}

type AnnNoDataflow struct {
	basicAnnotation
	goro    string
	goroann *AnnGoro
}

func (a AnnNoDataflow) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)
	return At(id_NO_DATAFLOW + " at " + npos.String())
}

func (a AnnNoDataflow) HasFocus() bool {
	return a.goro != ""
}

func (a AnnNoDataflow) Focused() AnnGoro {
	if a.goroann == nil {
		a.goroann = new(AnnGoro)
		*a.goroann = a.mgr.FindGoro(a.goro)
	}
	return *a.goroann
}

func (a AnnNoDataflow) Nodes() map[cfg.Node]struct{} {
	return commNodes(a.basicAnnotation)
}

type AnnPSet struct {
	basicAnnotation
	val ssa.Value
}

type AnnChan struct {
	basicAnnotation
	ch ssa.Value
}

func (a AnnChan) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)
	return At("Channel at " + npos.String() +
		"\n  name: " + idToStr(a.note.Args[0]) +
		"\n  SSA value: " + a.ch.String())
}

func (a AnnChan) Name() string {
	return idToStr(a.note.Args[0])
}

func commTransitiveNodes(a Annotation) map[cfg.Node]struct{} {
	nodes := a.Manager().NodesForNote(a.Note())
	commsuccs := make(map[cfg.Node]struct{})

	for node := range nodes {
		for cs := range node.CommTransitive() {
			commsuccs[cs] = struct{}{}
		}
	}

	return commsuccs
}

func commNodes(a Annotation) map[cfg.Node]struct{} {
	nodes := a.Manager().NodesForNote(a.Note())
	commsuccs := make(map[cfg.Node]struct{})

	for node := range nodes {
		if node.IsCommunicationNode() {
			commsuccs[node] = struct{}{}
		}
	}

	return commsuccs
}

func (a AnnPSet) Value() ssa.Value {
	return a.val
}

func (a AnnChan) Nodes() map[cfg.Node]struct{} {
	return commTransitiveNodes(a.basicAnnotation)
}

func (a AnnChan) Chan() ssa.Value {
	return a.ch
}

type AnnChanQuery struct {
	basicAnnotation
	prop        string
	val         L.Element
	chnote      Annotation
	gownerNote  Annotation
	focusedNote Annotation
	gowner      string
	focused     string
	ch          ssa.Value
}

func (a AnnChanQuery) Value() L.Element {
	return a.val
}

func (a AnnChanQuery) Chan() ssa.Value {
	return a.ch
}

func (a AnnChanQuery) Gowner() string {
	return a.gowner
}

func (a AnnChanQuery) Focused() string {
	return a.focused
}

func (a AnnChanQuery) Prop() string {
	return a.prop
}

// Returns related channel queries.
func (a1 AnnChanQuery) ChanRelated() []Annotation {
	fas := []Annotation{}
	for _, a2 := range a1.basicAnnotation.Related() {
		switch a2 := a2.(type) {
		case AnnChanQuery:
			if a1.ch == a2.ch {
				fas = append(fas, a2)
			}
		}
	}

	return fas
}

// Retrieves the channel annotation with the same channel name.
func (a *AnnChanQuery) ChanNote() AnnChan {
	if a.chnote != nil {
		return a.chnote.(AnnChan)
	}

	for _, a2 := range a.mgr.anns {
		a2, ok := a2.(AnnChan)
		if !ok {
			continue
		}

		if a.ch == a2.ch {
			a.chnote = a2
			return a2
		}
	}

	// Only reachable if no channel annotation was found.
	// Annotation creation should prevent such scenarios from happening.
	panic("Unreachable.")
}

// Retrieves the goroutine annotation for the goroutine
// that owns the channel. It should always succeed.
func (a AnnChanQuery) GownerNote() AnnGoro {
	return a.mgr.FindGoro(a.gowner)
}
func (a AnnChanQuery) HasOwner() bool {
	return a.gowner != ""
}

// Retrieves the goroutine annotation for the focused goroutine
// of that channel query. It should always succeed.
func (a AnnChanQuery) FocusedNote() AnnGoro {
	return a.mgr.FindGoro(a.focused)
}
func (a AnnChanQuery) IsFocused() bool {
	return a.focused != ""
}

func (a AnnChanQuery) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)
	var suffix string
	if a.HasOwner() {
		suffix += "\n  Channel owner: " + a.mgr.FindGoro(a.gowner).String()
	}
	if a.IsFocused() {
		suffix += "\n	 Focused goroutine: " + a.mgr.FindGoro(a.focused).String()
	}
	return At("Channel " + a.prop + " query at " + npos.String() +
		"\n  name: " + idToStr(a.note.Args[0]) +
		"\n  SSA value: " + a.ch.String() + " at " + fset.Position(a.ch.Pos()).String() +
		"\n  Abstract value: " + a.val.String() +
		suffix)
}

func (a AnnChanQuery) Nodes() map[cfg.Node]struct{} {
	nodes := a.basicAnnotation.mgr.NodesForNote(a.note)
	commsuccs := make(map[cfg.Node]struct{})

	for node := range nodes {
		for cs := range node.CommTransitive() {
			commsuccs[cs] = struct{}{}
		}
	}

	return commsuccs
}

// Annotation for naming spawning points
type AnnGo struct {
	basicAnnotation
	g cfg.Node
}

// Goroutine CFG node associated with spawning point.
func (a AnnGo) Go() cfg.Node {
	return a.g
}

// Annotation for creating specific goroutines.
// Goroutines consist of a chain of spawning points.
// The order is given left to right from the root goroutine.
type AnnGoro struct {
	basicAnnotation
	gos       []cfg.Node
	hardmatch bool
}

func (a AnnGoro) Goro() []cfg.Node {
	return a.gos
}

func (a AnnGoro) Name() string {
	return idToStr(a.note.Args[0])
}

func (a AnnGoro) String() string {
	str := a.Name()
	nodestrs := make([]string, 0, len(a.gos))
	for _, node := range a.gos {
		nodestrs = append(nodestrs, node.String())
	}

	return str + ": " + strings.Join(nodestrs, " ↝ ")
}

// Checks whether a goroutine structure is represented by
// a goroutine annotation (only takes control locations into account).
// If the goroutine annotation is set to hardmatch, then the full spawning
// point chain must match. Otherwise, only the inner-most chain must.
func (a AnnGoro) Matches(g defs.Goro) bool {
	i := len(a.gos) - 1
	for ; g != nil && i >= 0; g = g.Parent() {
		if g.CtrLoc().Node() != a.gos[i] {
			return false
		}
		i--
	}

	if a.hardmatch {
		return i < 0 && g == nil
	}
	return i < 0
}

type AnnFocused struct {
	basicAnnotation
	goro    string
	goroann *AnnGoro
}

func (a AnnFocused) Matches(g defs.Goro) bool {
	return a.Goro().Matches(g)
}

func (a AnnFocused) Goro() AnnGoro {
	if a.goroann == nil {
		a.goroann = new(AnnGoro)
		*a.goroann = a.mgr.FindGoro(a.goro)
	}

	return *a.goroann
}

func (a AnnFocused) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)

	return At("Focus on goroutine " + a.goro + " at " + npos.String())
}

type AnnFalseNegative struct {
	basicAnnotation
}

func (a AnnFalseNegative) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)

	return At("False negative at " + npos.String())
}

type AnnFalsePositive struct {
	basicAnnotation
}

func (a AnnFalsePositive) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)

	return At("False positive at " + npos.String())
}
