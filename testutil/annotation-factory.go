package testutil

import (
	"Goat/analysis/cfg"
	L "Goat/analysis/lattice"
	"fmt"
	"log"
	"strconv"
	"strings"

	"golang.org/x/tools/go/expect"
	"golang.org/x/tools/go/ssa"
)

var (
	id_PANICKED       = "panicked"
	id_GORO           = "goro"
	id_GO             = "go"
	id_CHAN           = "chan"
	id_CHAN_QUERY     = "chan_query"
	id_BLOCKS         = "blocks"
	id_MAY_RELEASE    = "releases"
	id_FALSE_POSITIVE = "fp"
	id_FALSE_NEGATIVE = "fn"
	id_ANALYSIS       = "analysis"
	QRY_CAP           = "cap"
	QRY_MULTIALLOC    = "multialloc"
	QRY_STATUS        = "status"
	QRY_BUFFER_F      = "bufferF"
	QRY_BUFFER_I      = "bufferI"
)

type interval = struct {
	L int
	H int
}

// Convert expect.Identifier to string.
func idToStr(x interface{}) string {
	return string(x.(expect.Identifier))
}

type annFactory struct{}

// Factory for creating annotation strings. Interpolate
// results with Go source code. Wrap multiple factory calls
// in the At function to concatenate multiple annotations
// on the same line and prefix with "//@ "
var Ann = annFactory{}

// Create a blocking annotation. It specifies that the analysis is
// expected to find the specified goroutine blocked at the control location
// of the annotated program point.
func (annFactory) Blocks(g string) string {
	return id_BLOCKS + "(" + g + ")"
}

// False negative tag.
func (annFactory) FalseNegative() string {
	return id_FALSE_NEGATIVE
}

// False positive tag.
func (annFactory) FalsePositive() string {
	return id_FALSE_POSITIVE
}

// Create a `may release` annotation. It specifies that the analysis is
// expected to not find a superlocation blocked at a control location
// at the annotated program point.
func (annFactory) MayRelease(g string) string {
	return id_MAY_RELEASE + "(" + g + ")"
}

// Provide with a list of `go` annotation names in the program to create a
// goroutine spawning chain. The "hard match" argument states whether the analysis goroutine
// should match the provided chain precisely, or only its inner-most components.
func (annFactory) Goro(name string, hardmatch bool, g ...string) string {
	return id_GORO + "(" + name + ", " + strconv.FormatBool(hardmatch) + "," + strings.Join(g, ", ") + ")"
}

// Create a channel annotation string. Will match a
// (random if more than one) "make chan" SSA value at the same line as the
// annotation.
func (annFactory) Chan(name string) string {
	return id_CHAN + "(" + name + ")"
}

// Create a go annotation string. Will match a (random if more than one) "go"
// SSA instruction with the given name. Can also be used as the same line as a
// function signature if wanting to name e. g. the main goroutine. It will
// then connect to the functio entry note.
func (annFactory) Go(name string) string {
	return id_GO + "(" + name + ")"
}

// Create a channel query annotation string, which works as follows:
//
// 1. It requires a channel name which matches one of the channel annotations in
// the program.
//
// 2. As with all annotations, it will attempt to match several control flow
// nodes to the annotation using the usual heuristic. However, because these control
// flow nodes are not guaranteed to be in superlocations in the analysis result,
// all of them are instead swapped with the communication nodes which are
// immediate transitive successors.
//
// 3. On the analysis result, it will find all superlocations that have a
// control location with the control flow node in the associated nodes set
// of the annotation.
//
// 4. It will then look in the memory for all allocation site
// locations using the "make chan" instruction of the channel annotation. For all
// such locations, it will match the expected value with the found one, via
// lattice element equality.
//
// Properties should be given by name using one of the QRY_X.
// If the provided value is not parsable, feedback will let you know which
// values are allowed for a given property.
func (annFactory) ChanQuery(
	ch string,
	prop string,
	val interface{},
) string {
	return Ann.FocusedChanQuery(ch, prop, val, "", "")
}

// Create a channel query annotation string where the channel
// is owned (was instantiated) by a specific goroutine, and the program
// point is only considered for the given focused goroutine.
// It works as follows:
//
// 1. It requires a channel name which matches one of the channel annotations in
// the program by name. The owner goroutine should also match a Go annotation by
// name in the program.
//
// 2. As with all annotations, it will attempt to match several control flow
// nodes to the annotation using the usual heuristic. However, because these control
// flow nodes are not guaranteed to be in superlocations in the analysis result,
// all of them are instead swapped with the communication nodes which are
// immediate transitive successors.
//
// 3. On the analysis result, it will find all superlocations that have the
// focused goroutine at a control location with the control flow node in
// the associated nodes set of the annotation.
//
// 4. It will then look in the memory for all allocation site
// locations using the "make chan" instruction of the channel annotation and
// owned by an equivalent goroutine. A goroutine is considered to "own" the
// channel if its annotation matches the allocation site's goroutine.
// For all such locations, it will compare the expected value
// with the real one via lattice element equality.
//
// Properties should be given by name using one of the QRY_X.
// If the provided value is not parsable, feedback will let you know which
// values are allowed for a given property.
func (annFactory) FocusedChanQuery(
	ch string,
	prop string,
	val interface{},
	gowner string,
	focused string,
) string {
	var allowedVals string
	query := id_CHAN_QUERY + "(" + ch + ", " + prop + ", "
	suffix := ")"
	if focused != "" {
		suffix = ", " + focused + suffix
	}
	if gowner != "" {
		suffix = ", " + gowner + suffix
	}
	quote := func(str string) string {
		return "\"" + str + "\""
	}

	// Check for sanitary input
	switch prop {
	case QRY_MULTIALLOC:
		if val, ok := val.(bool); ok {
			return query + strconv.FormatBool(val) + suffix
		}
		allowedVals += strings.Join([]string{
			"  - true for multi-allocated values",
			"  - false for singly-allocated values",
		}, "\n") + "\n"
	case QRY_CAP:
		switch val := val.(type) {
		case string:
			if val == "top" || val == "bot" {
				return query + quote(val) + suffix
			}
		case int:
			if val >= 0 {
				return query + strconv.Itoa(val) + suffix
			}
		}

		allowedVals += strings.Join([]string{
			"  - top for unknown capacity",
			"  - bot for nil channel capacity",
			"  - Natural numbers for constant capacity",
		}, "\n") + "\n"
	case QRY_BUFFER_F:
		switch val := val.(type) {
		case string:
			if val == "top" || val == "bot" {
				return query + quote(val) + suffix
			}
		case int:
			if val >= 0 {
				return query + strconv.Itoa(val) + suffix
			}
		}

		allowedVals += strings.Join([]string{
			"  - top for unknown buffer size",
			"  - bot for nil channel buffer size",
			"  - Natural numbers for constant buffer sizes",
		}, "\n") + "\n"
	case QRY_BUFFER_I:
		switch val := val.(type) {
		case string:
			if val == "top" || val == "bot" {
				return query + quote(val) + suffix
			}
		case interval:
			if val.L >= 0 && val.H >= 0 {
				return query + strconv.Itoa(val.L) + ", " + strconv.Itoa(val.H) + suffix
			}
		}

		allowedVals += strings.Join([]string{
			"  - top for unknown buffer size interval",
			"  - bot for nil channel buffer size intervals",
			fmt.Sprintf("  - %T, where l >= 0 and h >= 0 for known interval bounds.", interval{}),
		}, "\n") + "\n"
	case QRY_STATUS:
		switch val := val.(type) {
		case string:
			if val == "top" || val == "bot" {
				return query + quote(val) + suffix
			}
		case bool:
			return query + strconv.FormatBool(val) + suffix
		}

		allowedVals += strings.Join([]string{
			"  - top for unknown channel status",
			"  - bot for nil channel status",
			"  - true for OPEN channels",
			"  - false for CLOSED channels",
		}, "\n") + "\n"
	}

	panic(fmt.Sprintf(
		"Invalid query on %s for %s. Got %s : %T. Allowed values:\n"+
			allowedVals,
		ch,
		prop,
		val,
		val))
}

// Prefixes a sequence of strings with "//@ "
func At(anns ...string) string {
	ann := "//@ " + strings.Join(anns, ", ")
	return ann
}

func (mgr NotesManager) CreateAnnotation(note *expect.Note) Annotation {
	basic := basicAnnotation{note, mgr}

	// Assumes only one MakeChan operation is available per line.
	findChan := func(note *expect.Note) ssa.Value {
		for node := range mgr.NodesForNote(note) {
			node, ok := node.(*cfg.SSANode)
			if !ok {
				continue
			}
			chn, ok := node.Instruction().(*ssa.MakeChan)
			if !ok {
				continue
			}

			return chn
		}
		return nil
	}

	// Assumes only one go instruction is available per line.
	findGo := func(note *expect.Note) cfg.Node {
		// Collect all CFG nodes for the note.
		nodes := mgr.NodesForNote(note)

		// Favor Go instructions.
		for node := range nodes {
			switch node := node.(type) {
			case *cfg.SSANode:
				if _, ok := node.Instruction().(*ssa.Go); !ok {
					continue
				}
				return node
			}
		}
		// Check for function entry next.
		for node := range nodes {
			if _, ok := node.(*cfg.FunctionEntry); ok {
				return node
			}
		}

		return nil
	}

	findGoNote := func(goname string) (*expect.Note, bool) {
		return mgr.FindNote(func(n *expect.Note) bool {
			if len(n.Args) != 1 {
				return false
			}
			return n.Name == "go" && idToStr(n.Args[0]) == goname
		})
	}

	switch note.Name {
	case id_FALSE_NEGATIVE:
		return AnnFalseNegative{basic}
	case id_FALSE_POSITIVE:
		return AnnFalsePositive{basic}
	case id_PANICKED:
		return AnnPanicked{basic}
	case id_BLOCKS:
		if len(note.Args) > 0 {
			return AnnBlocks{basic, idToStr(note.Args[0]), nil}
		}
		return AnnBlocks{basicAnnotation: basic}
	case id_MAY_RELEASE:
		if len(note.Args) > 0 {
			return AnnReleases{basic, idToStr(note.Args[0]), nil}
		}
		return AnnReleases{basicAnnotation: basic}
	case id_ANALYSIS:
		// Convert old analysis notes to blocks/releases notes
		// TODO: Maybe it's better to just do it in the source files directly.
		if note.Args[0].(bool) {
			return AnnReleases{basicAnnotation: basic}
		} else {
			return AnnBlocks{basicAnnotation: basic}
		}
	case id_CHAN:
		if chn := findChan(note); chn != nil {
			for _, ann2 := range mgr.anns {
				switch ann2 := ann2.(type) {
				case AnnChan:
					if ann2.Name() != idToStr(note.Args[0]) {
						continue
					}
					panic(fmt.Sprintf(
						"Multiple channel annotations found using the same channel name:\n  -%s\n  -%s",
						basic,
						ann2,
					))
				}
			}
			return AnnChan{
				basic,
				chn,
			}
		}
		panic(fmt.Sprintf(
			"Channel annotation found, but no MakeChan SSA node: %s",
			basic,
		))
	case id_GO:
		if g := findGo(note); g != nil {
			return AnnGo{basic, g}
		}
		panic(fmt.Sprintf(
			"Goroutine annotation found, but no `go` instruction or function entry node to associate with: %s",
			basic,
		))
	case id_GORO:
		hardmatch := note.Args[1].(bool)
		gos := make([]cfg.Node, 0, len(note.Args)-2)

		for _, g := range note.Args[2:] {
			name := idToStr(g)
			if name == "_root" {
				entries := mgr.loadRes.Cfg.GetEntries()
				if len(entries) != 1 {
					panic(fmt.Errorf("CFG does not have exactly one entry: %v", entries))
				}
				gos = append(gos, entries[0])
			} else {
				gonote, found := findGoNote(name)
				if !found {
					panic(fmt.Sprintf(
						"Used inexistent go spawn annotation node: %s",
						basic,
					))
				}
				gos = append(gos, findGo(gonote))
			}
		}

		return AnnGoro{
			basic,
			gos,
			hardmatch,
		}

	case id_CHAN_QUERY:
		// TODO: It might be nice to be able to query channels without annotating
		// the allocation site. For instance you could put an annotation on a
		// communication CFG node (Send, Rcv, Select(variants)), and the query
		// would act on the channel value used for communication.
		chNote, found := mgr.FindNote(func(n *expect.Note) bool {
			if len(n.Args) != 1 {
				return false
			}
			qch := idToStr(note.Args[0])
			ch := idToStr(n.Args[0])
			return n.Name == id_CHAN && qch == ch
		})

		if !found {
			panic(fmt.Sprintf(
				"Channel query for inexistent channel annotation node: %s",
				basic,
			))
		}

		prop := idToStr(note.Args[1])
		ch := findChan(chNote)

		var val L.Element
		gownerIndex := 3
		focusedIndex := 4
		switch prop {
		case QRY_MULTIALLOC:
			val = L.Create().Element().TwoElement(note.Args[2].(bool))
		case QRY_CAP:
			switch qval := note.Args[2].(type) {
			case string:
				if qval == "top" {
					val = L.Create().Lattice().FlatInt().Top()
				}
				if qval == "bot" {
					val = L.Create().Lattice().FlatInt().Bot()
				}
			case int64:
				val = L.Create().Element().FlatInt((int)(qval))
			}
		case QRY_STATUS:
			switch qval := note.Args[2].(type) {
			case string:
				if qval == "top" {
					val = L.Create().Lattice().ChannelInfo().Status().Top()
				}
				if qval == "bot" {
					val = L.Create().Lattice().ChannelInfo().Status().Bot()
				}
			case bool:
				if qval {
					val = L.Consts().Open()
				} else {
					val = L.Consts().Closed()
				}
			}
		case QRY_BUFFER_F:
			switch qval := note.Args[2].(type) {
			case string:
				if qval == "top" {
					val = L.Create().Lattice().FlatInt().Top()
				}
				if qval == "bot" {
					val = L.Create().Lattice().FlatInt().Bot()
				}
			case int64:
				val = L.Create().Element().FlatInt((int)(qval))
			}
		case QRY_BUFFER_I:
			switch qval := note.Args[2].(type) {
			case string:
				if qval == "top" {
					val = L.Create().Lattice().Interval().Top()
				}
				if qval == "bot" {
					val = L.Create().Lattice().Interval().Bot()
				}
			case int64:
				low := qval
				gownerIndex = 4
				focusedIndex = 5
				high := note.Args[3].(int64)
				val = L.Create().Element().IntervalFinite((int)(low), (int)(high))
			}
		}
		if val == nil {
			log.Fatal("???", note)
		}

		var gowner string
		var focused string
		pred := func(index int) func(n *expect.Note) bool {
			return func(n *expect.Note) bool {
				if len(n.Args) < 1 {
					return false
				}
				return n.Name == id_GORO &&
					idToStr(n.Args[0]) == idToStr(note.Args[index])
			}
		}
		if len(note.Args) > gownerIndex {
			gownerNote, found := mgr.FindNote(pred(gownerIndex))
			if !found {
				panic(fmt.Sprintf(
					"Owner goroutine for channel query does not exist: %s",
					basic,
				))
			}
			gowner = idToStr(gownerNote.Args[0])
		}
		if len(note.Args) > focusedIndex {
			focusedNote, found := mgr.FindNote(pred(focusedIndex))

			if !found {
				panic(fmt.Sprintf(
					"Focused goroutine for channel query does not exist: %s",
					basic,
				))
			}
			focused = idToStr(focusedNote.Args[0])
		}

		return AnnChanQuery{
			basic,
			prop,
			val,
			nil,
			nil,
			nil,
			gowner,
			focused,
			ch,
		}
	}

	return basic
}
