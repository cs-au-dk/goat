package testutil

import (
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	"fmt"
	"testing"

	"golang.org/x/tools/go/expect"
)

type NotesManager struct {
	anns  map[*expect.Note]Annotation
	notes []*expect.Note

	// Book-keeping of notes on the same line
	related map[*expect.Note]map[*expect.Note]struct{}
	loadRes LoadResult
}

func MakeNotesManager(
	t *testing.T,
	loadRes LoadResult) (n NotesManager) {
	n.loadRes = loadRes
	n.anns = make(map[*expect.Note]Annotation)

	for _, file := range loadRes.MainPkg.Syntax {
		notes, err := expect.ExtractGo(loadRes.Prog.Fset, file)
		if err != nil {
			t.Fatal(err)
		}

		n.notes = append(n.notes, notes...)
	}

	fset := loadRes.Prog.Fset

	n.related = make(map[*expect.Note]map[*expect.Note]struct{})
	for _, note1 := range n.notes {
		if _, found := n.related[note1]; !found {
			n.related[note1] = make(map[*expect.Note]struct{})
		}

		npos1 := fset.Position(note1.Pos)

		for _, note2 := range n.notes {
			if note1 == note2 {
				continue
			}

			npos2 := fset.Position(note2.Pos)

			if npos1.Filename == npos2.Filename &&
				npos1.Line == npos2.Line {
				n.related[note1][note2] = struct{}{}
			}
		}
	}

	for _, note := range n.notes {
		n.anns[note] = n.CreateAnnotation(note)
	}
	return
}

func (n NotesManager) ForEachNote(
	do func(i int, note *expect.Note),
) {
	for i, note := range n.notes {
		do(i, note)
	}
}

func (n NotesManager) ForEachAnnotation(do func(a Annotation)) {
	for _, a := range n.anns {
		do(a)
	}
}

func (n NotesManager) AnnotationOf(note *expect.Note) Annotation {
	return n.anns[note]
}

func (n NotesManager) String() (str string) {
	str = "Note manager found the following notes:\n\n"
	for _, note := range n.notes {
		pos := n.loadRes.Prog.Fset.Position(note.Pos)

		str += fmt.Sprintf("%s(%v) at position: %s, with the following CFG nodes:\n", note.Name, note.Args, pos)
		for node := range n.NodesForNote(note) {
			str += "- " + node.String() + "\n"
		}
		str += "Annotation:\n" + n.anns[note].String() + "\n"
	}

	return
}

func (n NotesManager) LoadResult() LoadResult {
	return n.loadRes
}

func (n NotesManager) NodesForNote(note *expect.Note) map[cfg.Node]struct{} {
	fset := n.loadRes.Prog.Fset
	npos := fset.Position(note.Pos)

	return n.loadRes.Cfg.FindAll(func(node cfg.Node) bool {
		pos := fset.Position(node.Pos())
		return pos.Filename == npos.Filename && pos.Line == npos.Line && pos.Column <= npos.Column
	})
}

func (n NotesManager) Notes() []*expect.Note {
	return n.notes
}

func (n NotesManager) Annotations() map[*expect.Note]Annotation {
	return n.anns
}

func (n NotesManager) FindNote(find func(*expect.Note) bool) (*expect.Note, bool) {
	for _, note := range n.notes {
		if find(note) {
			return note, true
		}
	}
	return nil, false
}

func (n NotesManager) FindAllAnnotations(pred func(Annotation) bool) annList {
	res := []Annotation{}

	for _, ann := range n.anns {
		if pred(ann) {
			res = append(res, ann)
		}
	}

	return res
}

func (n NotesManager) FindAllNotes(find func(*expect.Note) bool) (notes map[*expect.Note]struct{}) {
	notes = make(map[*expect.Note]struct{})

	for _, note := range n.notes {
		if find(note) {
			notes[note] = struct{}{}
		}
	}
	return
}

func (n NotesManager) NoteForCtrLoc(cl defs.CtrLoc) (*expect.Note, bool) {
	note, found := n.NoteForNode(cl.Node())
	if !found {
		return nil, false
	}

	_, panicked := n.anns[note].Related().Find(func(a Annotation) bool {
		_, ok := a.(AnnPanicked)
		return ok
	})

	switch {
	case cl.Panicked() && panicked:
		fallthrough
	case !cl.Panicked() && !panicked:
		return note, true
	}

	return nil, false
}

func (n NotesManager) NoteForNode(node cfg.Node) (*expect.Note, bool) {
	fset := n.loadRes.Prog.Fset
	pos := fset.Position(node.Pos())

	return n.FindNote(func(note *expect.Note) bool {
		npos := fset.Position(note.Pos)
		return pos.Filename == npos.Filename && pos.Line == npos.Line && pos.Column <= npos.Column
	})
}

func (n NotesManager) NotesForNode(node cfg.Node) map[*expect.Note]struct{} {
	fset := n.loadRes.Prog.Fset
	pos := fset.Position(node.Pos())

	return n.FindAllNotes(func(n *expect.Note) bool {
		npos := fset.Position(n.Pos)
		return pos.Filename == npos.Filename && pos.Line == npos.Line && pos.Column <= npos.Column
	})
}

func (n NotesManager) NodeHasAnn(node cfg.Node, ann Annotation) bool {
	notes := n.NotesForNode(node)

	for note := range notes {
		return note == ann.Note()
	}
	return false
}

func (n NotesManager) FindGoro(name string) AnnGoro {
	for node, ann := range n.anns {
		ann, ok := ann.(AnnGoro)
		if !ok {
			continue
		}

		if idToStr(node.Args[0]) == name {
			return ann
		}
	}

	panic(fmt.Sprintf(
		"Goroutine annotation named %s not found", name,
	))
}

func (n NotesManager) OrphansToAnnotations(orphans map[defs.Superloc]map[defs.Goro]struct{}) map[defs.Superloc]annList {
	goros := n.FindAllAnnotations(func(a Annotation) bool {
		_, ok := a.(AnnGoro)
		return ok
	})

	res := make(map[defs.Superloc]annList)

	for sl := range orphans {
		sl.ForEach(func(g defs.Goro, cl defs.CtrLoc) {
			goro, found := goros.Find(func(a Annotation) bool {
				return a.(AnnGoro).Matches(g)
			})
			if found {
				res[sl] = append(res[sl], goro)
			}
		})
	}

	return res
}
