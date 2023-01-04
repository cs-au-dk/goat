package upfront

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/ssa"
)

// ChannelOp aggregates all possible
// instructions associated with a MakeChan
// instruction in the source.
type ChannelOp struct {
	Buffer  ssa.Value
	Send    []ssa.Instruction
	Receive []ssa.Instruction
	Close   []ssa.Instruction
	Make    ssa.Instruction
}

func (op *ChannelOp) String() (opStr string) {
	if op.Make != nil {
		opStr = fmt.Sprintf("Created at: %s:%d\n", op.Make.String(), op.Make.Pos())
	}
	if buffer := op.BufferLabel(); buffer != "" {
		opStr = opStr + "\nBuffer: " + buffer + "\n"
	}
	if len(op.Send) > 0 {
		opStr = opStr + "\nSends: "
		strs := []string{}
		for _, send := range op.Send {
			strs = append(strs, send.String())
		}
		opStr = opStr + strings.Join(strs, ", ")
	}
	if len(op.Receive) > 0 {
		opStr = opStr + "\nReceives: "
		strs := []string{}
		for _, rcv := range op.Receive {
			strs = append(strs, rcv.String())
		}
		opStr = opStr + strings.Join(strs, ", ")
	}
	if len(op.Close) > 0 {
		opStr = opStr + "\nCloses: "
		strs := []string{}
		for _, close := range op.Close {
			strs = append(strs, close.String())
		}
		opStr = opStr + strings.Join(strs, ", ")
	}
	return
}

// BufferLabel formats the buffer size to a string.
func (op *ChannelOp) BufferLabel() string {
	if opts.Extended() {
		switch buf := op.Buffer.(type) {
		case *ssa.Const:
			defer func() {
				if x := recover(); x != nil {
					panic(fmt.Sprintf("MakeChan SSA Size is a non-integer constant:\n%s", x))
				}
			}()
			value := buf.Int64()
			if value == 0 {
				return ""
			}
			return fmt.Sprintf("%d", value)
		default:
			return buf.String()
		}
	}
	return ""
}
