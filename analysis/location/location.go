package location

import (
	"go/types"

	"github.com/cs-au-dk/goat/utils"

	"github.com/fatih/color"
	"golang.org/x/tools/go/ssa"
)

// colorize is used for pretty-printing.
var colorize = struct {
	Site        func(...interface{}) string
	Cons        func(...interface{}) string
	Context     func(...interface{}) string
	Nil         func(...interface{}) string
	Index       func(...interface{}) string
	Instruction func(...interface{}) string
}{
	Site: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiGreen).SprintFunc())(is...)
	},
	Cons: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiYellow).SprintFunc())(is...)
	},
	Context: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiBlue).SprintFunc())(is...)
	},
	Nil: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiRed).SprintFunc())(is...)
	},
	Index: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiCyan).SprintFunc())(is...)
	},
	Instruction: func(is ...interface{}) string {
		return utils.CanColorize(color.New(color.FgHiWhite, color.Faint).SprintFunc())(is...)
	},
}

// phasher is a short-hand for a pointer hasher.
var phasher = utils.PointerHasher[any]{}

// Context is given to allocation sites.
//
// TODO: This type should be defined somewhere else with a suitable implementation
// TODO: We probably want more context.
type Context = *ssa.Function

// A location points to something (or nothing) in the abstract memory.
// It can be an allocation site, a global variable, or a field of a struct.
// TODO: Consider renaming to Pointer
type Location interface {
	Hash() uint32
	Equal(Location) bool
	String() string
	GetSite() (site ssa.Value, ok bool)
	Type() types.Type
	Position() string
}

// LocationHasher needed for immutable.Map
type LocationHasher struct{}

func (LocationHasher) Hash(key Location) uint32 {
	return key.Hash()
}

func (LocationHasher) Equal(a, b Location) bool {
	return a.Equal(b)
}

// AddressableLocation is implemented by pointers bound directly in
// abstract memory. It excludes field addresses and the nil pointer
// from such lookups.
type AddressableLocation interface {
	Location
	addressableTag()
}

// addressable is a property embedded by all addressable heap locations.
type addressable struct{}

func (addressable) addressableTag() {}
