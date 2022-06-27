package testutil

import (
	"Goat/analysis/cfg"
	"fmt"
	"strings"

	"golang.org/x/tools/go/expect"
)

type basicAnnotation struct {
	note *expect.Note
	mgr  NotesManager
}

func (a basicAnnotation) Note() *expect.Note {
	return a.note
}

func (a basicAnnotation) Name() string {
	return a.note.Name
}

func (a basicAnnotation) Manager() NotesManager {
	return a.mgr
}

type annList []Annotation

// Returns all annotations found on the same line as the given annotation.
func (a1 basicAnnotation) Related() annList {
	// Arbitrary capacity.
	as := make([]Annotation, 0, 5)

	for n2 := range a1.mgr.related[a1.note] {
		as = append(as, a1.mgr.AnnotationOf(n2))
	}

	return as
}

func (a basicAnnotation) Nodes() map[cfg.Node]struct{} {
	return a.mgr.NodesForNote(a.note)
}

func (a basicAnnotation) String() string {
	fset := a.mgr.loadRes.Prog.Fset
	npos := fset.Position(a.note.Pos)
	args := make([]string, 0, len(a.note.Args))
	for _, arg := range a.note.Args {
		args = append(args, fmt.Sprintf("%v", arg))
	}
	return "//@ Basic annotation: " + a.note.Name + "(" +
		strings.Join(args, ", ") + ") at " + npos.String()
}

func (a basicAnnotation) Panicked() bool {
	return a.Related().Exists(func(a Annotation) bool {
		_, ok := a.(AnnPanicked)
		return ok
	})
}

func (a basicAnnotation) FalseNegative() bool {
	return a.Related().Exists(func(a Annotation) bool {
		_, ok := a.(AnnFalseNegative)
		return ok
	})
}

func (a basicAnnotation) FalsePositive() bool {
	return a.Related().Exists(func(a Annotation) bool {
		_, ok := a.(AnnFalsePositive)
		return ok
	})
}

func (la annList) Filter(pred func(Annotation) bool) annList {
	res := make([]Annotation, 0, len(la))

	for _, ann := range la {
		if pred(ann) {
			res = append(res, ann)
		}
	}

	return res
}

func (la annList) Find(pred func(Annotation) bool) (Annotation, bool) {
	for _, ann := range la {
		if pred(ann) {
			return ann, true
		}
	}
	return nil, false
}

func (la annList) Exists(pred func(Annotation) bool) bool {
	_, found := la.Find(pred)
	return found
}

func (la annList) ForEach(do func(Annotation)) {
	for _, ann := range la {
		do(ann)
	}
}
