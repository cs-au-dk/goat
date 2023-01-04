package location

import (
	"fmt"
	"go/types"
	"log"
	"regexp"

	"github.com/benbjohnson/immutable"
	"github.com/cs-au-dk/goat/utils"
	"golang.org/x/tools/go/ssa"
)

// LocalLocation represents the symbolic address in memory of an SSA register.
type LocalLocation struct {
	addressable
	Goro     utils.Hashable
	Context  Context
	Name     string // The name of the variable
	DeclLine int64  // The source line the variable was declared on (used to disambiguate multiple variables with the same name)
	// TODO: The debugger frontend can (currently) not supply the correct ssa.Value as the Site.
	// This messes up the Equal function when the abstract interpreter _can_ supply the correct Site
	// when reconstructing the LocalLocation.
	// If we need the Site member we can either: extend the debugger frontend, or, exclude the
	// Site field from the Equal-check.
	Site ssa.Value
}

func (l LocalLocation) Equal(ol Location) bool {
	o, ok := ol.(LocalLocation)
	return ok && l == o
}

func (l LocalLocation) Position() string {
	if l.Site.Parent() != nil {
		return l.Site.Parent().Prog.Fset.Position(l.Site.Pos()).String()
	}

	return ""
}

func (l LocalLocation) Hash() uint32 {
	ihasher := immutable.NewHasher(l.DeclLine)
	shasher := immutable.NewHasher(l.Name)

	return utils.HashCombine(
		l.Goro.Hash(),
		ihasher.Hash(l.DeclLine),
		shasher.Hash(l.Name),
		phasher.Hash(l.Context),
	)
}

func (l LocalLocation) String() string {
	if l.Site != nil {
		return fmt.Sprintf("‹%s: %s %s(%d) = %s›",
			l.Goro, colorize.Context(l.Context),
			colorize.Site(l.Name),
			l.DeclLine,
			colorize.Instruction(l.Site.String()))
	}
	return fmt.Sprintf("‹%s: %s %s(%d)›",
		l.Goro, colorize.Context(l.Context),
		colorize.Site(l.Name),
		l.DeclLine)
}

// GetSite retrieves the SSA instruction where the local register is assigned.
func (l LocalLocation) GetSite() (ssa.Value, bool) {
	return l.Site, l.Site != nil
}

func (l LocalLocation) Type() types.Type {
	if l.Site == nil {
		return nil
	}
	return l.Site.Type()
}

var registerNameRegexp = regexp.MustCompile(`^t\d+$`)

// LocationFromSSAValue creates a named local location from a given SSA register.
func LocationFromSSAValue(g utils.Hashable, val ssa.Value) LocalLocation {
	var name string

	switch val := val.(type) {
	case *ssa.FreeVar:
		name = val.Name()

	case *ssa.Global:
		panic(fmt.Errorf("do not call LocationFromSSAValue with a Global! %v", val))

	case *ssa.Parameter:
		// Prefix with "$" because the function will automatically make a local variable
		// with the same name (and reassign the parameter to that)
		name = "$" + val.Name()

	default:
		regname := val.Name()
		// TODO: If this regexp match is expensive (in profiling), we can
		// transform it into a plain loop over the characters in the name.
		if !registerNameRegexp.MatchString(regname) {
			log.Fatalf("%v does not correspond to a virtual register (%v)", regname, val)
		}

		// TODO: Register names are not guaranteed to be unique within a function
		// Register names should be unique within a block, though.
		// TODO: Possible fix? (Less pretty strings though)
		// ret.Name = "$" + utils.SSAValString(val)
		// Old mechanism, for posterity.
		name = "$" + regname
	}

	return LocalLocation{
		Goro:     g,
		Context:  val.Parent(),
		Name:     name,
		DeclLine: int64(val.Parent().Prog.Fset.Position(val.Pos()).Line),
		Site:     val,
	}
}

// ReturnLocation constructs a synthetic location for the return value of a function.
// This location is used by callers to retrieve the result.s
func ReturnLocation(g utils.Hashable, fun *ssa.Function) LocalLocation {
	return LocalLocation{
		Goro:     g,
		Context:  fun,
		Name:     "$return",
		DeclLine: int64(fun.Prog.Fset.Position(fun.Pos()).Line),
	}
}
