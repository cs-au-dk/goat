package upfront

import (
	"fmt"
	"path/filepath"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

type channelAliasingInfo struct {
	Location            string
	MaxChanPtsToSetSize int
}

var ChAliasingInfo = &channelAliasingInfo{}

func (ch *channelAliasingInfo) update(labels []*pointer.Label, i ssa.Instruction) {
	size := len(labels)
	if ch.MaxChanPtsToSetSize < size {
		ch.MaxChanPtsToSetSize = size
		pos := i.Parent().Prog.Fset.Position(i.Pos())
		ch.Location = fmt.Sprintf("%s/%s:%d @@ {", i.Parent().Package().Pkg.Path(), filepath.Base(pos.Filename), pos.Line)

		for _, label := range labels {
			val := label.Value()
			vpos := val.Parent().Prog.Fset.Position(val.Pos())

			ch.Location += fmt.Sprintf("%s/%s:%d, ", val.Parent().Package().Pkg.Path(), filepath.Base(vpos.Filename), vpos.Line)
		}
		ch.Location += "}"
	}
}
