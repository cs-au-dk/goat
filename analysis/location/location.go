package location

import (
	"Goat/analysis/upfront"
	"Goat/utils"
	"fmt"
	"go/types"
	"log"
	"regexp"

	"github.com/benbjohnson/immutable"
	"github.com/fatih/color"
	"golang.org/x/tools/go/ssa"
)

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

// TODO: This type should be defined somewhere else with a suitable implementation
// TODO: We probably want more context
type Context = *ssa.Function

// A location points to something (or nothing) in the abstract memory.
// It can be a concrete memory location, an allocation site, a global
// variable, or a field of a struct.
// TODO: Consider renaming to Pointer
type Location interface {
	Hash() uint32
	Equal(Location) bool
	String() string
	GetSite() (site ssa.Value, ok bool)
	Type() types.Type
	Position() string
}

/* LocationHasher needed for immutable.Map */
type LocationHasher struct{}

func (LocationHasher) Hash(key Location) uint32 {
	return key.Hash()
}

func (LocationHasher) Equal(a, b Location) bool {
	return a.Equal(b)
}

// Tagging interface used to tag pointers that can be used to look up
// directly in the abstract memory. Used to exclude field addresses
// and the nil pointer from such lookups. Using a tagging interface
// like this lets the type system help us avoid mistakes.
type AddressableLocation interface {
	Location
	addressableTag()
}

type aTag struct{}

func (aTag) addressableTag() {}

/* A location of a global variable */
type GlobalLocation struct {
	aTag
	Site *ssa.Global
}

func (l GlobalLocation) Equal(ol Location) bool {
	o, ok := ol.(GlobalLocation)
	return ok && l == o
}

func (l GlobalLocation) Position() string {
	if l.Site.Pkg != nil {
		return l.Site.Pkg.Prog.Fset.Position(l.Site.Pos()).String()
	}

	return ""
}

func (l GlobalLocation) Hash() uint32 {
	phasher := utils.PointerHasher{}
	return phasher.Hash(l.Site)
}

func (l GlobalLocation) String() string {
	return colorize.Cons("Global") + "(" +
		colorize.Site(l.Site.Name()) + ")"
}

func (l GlobalLocation) GetSite() (ssa.Value, bool) {
	return l.Site, l.Site != nil
}

func (l GlobalLocation) Type() types.Type {
	if l.Site == nil {
		return nil
	}
	return l.Site.Type()
}

/* An allocation site location */
type AllocationSiteLocation struct {
	aTag
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
	phasher := utils.PointerHasher{}

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
	// if l.Site.Parent() != nil && l.Site.Parent().Prog != nil {
	// 	pos = " at" + l.Site.Parent().Prog.Fset.Position(l.Site.Pos()).String()
	// }

	return fmt.Sprintf("‹%s:%s %s%s›",
		l.Goro,
		ctx,
		name,
		pos)
}

func (l AllocationSiteLocation) GetSite() (ssa.Value, bool) {
	return l.Site, l.Site != nil
}

func (l AllocationSiteLocation) Type() types.Type {
	if l.Site == nil {
		return nil
	}
	return l.Site.Type()
}

/* A location of a local variable */
type LocalLocation struct {
	aTag
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
	phasher := utils.PointerHasher{}

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

func ReturnLocation(g utils.Hashable, fun *ssa.Function) LocalLocation {
	return LocalLocation{
		Goro:     g,
		Context:  fun,
		Name:     "$return",
		DeclLine: int64(fun.Prog.Fset.Position(fun.Pos()).Line),
	}
}

/* Location of a field of a struct */
type FieldLocation struct {
	Base  Location
	Index int
}

func (l FieldLocation) Hash() uint32 {
	ihasher := immutable.NewHasher(l.Index)
	return utils.HashCombine(l.Base.Hash(), ihasher.Hash(l.Index))
}

func (l FieldLocation) Equal(ol Location) bool {
	o, ok := ol.(FieldLocation)
	return ok && l == o
}

func (l FieldLocation) Position() string {
	return l.Base.Position()
}

func (l FieldLocation) String() string {
	if l.Index == -2 {
		return fmt.Sprintf("%s"+colorize.Index("[*]"), l.Base)
	}
	return fmt.Sprintf("%s.("+colorize.Index("%d")+")", l.Base, l.Index)
}

func (l FieldLocation) GetSite() (ssa.Value, bool) {
	return nil, false
}

// Returns the nesting level of a field location
func (l FieldLocation) NestingLevel() (res int) {
	switch bl := l.Base.(type) {
	case FieldLocation:
		return res + bl.NestingLevel()
	}

	return 0
}

func (l FieldLocation) Type() types.Type {
	ptyp := l.Base.Type()

	// The base has to have either pointer or slice type
	switch utyp := ptyp.Underlying().(type) {
	case *types.Pointer:
		ptyp = utyp.Elem()
	case *types.Slice:
		if l.Index != -2 {
			log.Fatalln("???")
		}
		return types.NewPointer(utyp.Elem())
	default:
		log.Fatalln("Unexpected underlying type:", utyp)
	}

	switch ptyp := ptyp.Underlying().(type) {
	case *types.Tuple:
		// Can the ssa ever construct a pointer to a tuple field?
		// Isn't ssa.Extract always used for tuples?
		return types.NewPointer(ptyp.At(l.Index).Type())
	case *types.Struct:
		return types.NewPointer(ptyp.Field(l.Index).Type())
	case *types.Array:
		if l.Index != -2 {
			log.Fatalln("???")
		}
		return types.NewPointer(ptyp.Elem())
	default:
		panic(fmt.Sprintf("Field location %s base has type %s", l, ptyp))
	}
}

// Location of an element in an array.
// The analysis does not discern between different elements of the array, so we don't need an index.
type IndexLocation struct {
	Base Location
}

func (l IndexLocation) Hash() uint32 {
	// Prevent a guaranteed collision with the base.
	return utils.HashCombine(l.Base.Hash(), 42)
}

func (l IndexLocation) Equal(ol Location) bool {
	o, ok := ol.(IndexLocation)
	return ok && l == o
}

func (l IndexLocation) Position() string {
	return l.Base.Position()
}

func (l IndexLocation) String() string {
	return fmt.Sprintf("[%s]", l.Base)
}

func (l IndexLocation) GetSite() (ssa.Value, bool) {
	return nil, false
}

func (l IndexLocation) Type() types.Type {
	ptyp := l.Base.Type()

	switch ptyp := ptyp.Underlying().(type) {
	case *types.Array:
		return ptyp.Elem()
	case *types.Slice:
		return ptyp.Elem()
	default:
		panic(fmt.Sprintf("Index location %s base has type %s", l, ptyp))
	}
}

// Function pointer contains an *ssa.Function. Used for function values that do not need closures.
// See absint.evaluateSSA comment.
type FunctionPointer struct {
	Fun *ssa.Function
}

func (fp FunctionPointer) Hash() uint32 {
	phasher := utils.PointerHasher{}
	return phasher.Hash(fp.Fun)
}
func (fp FunctionPointer) Position() string {
	return fp.Fun.Parent().Prog.Fset.Position(fp.Fun.Pos()).String()
}

func (fp FunctionPointer) Equal(ol Location) bool {
	o, ok := ol.(FunctionPointer)
	return ok && fp == o
}

func (fp FunctionPointer) String() string {
	return fmt.Sprintf("Function(%s)", fp.Fun)
}

func (l FunctionPointer) GetSite() (ssa.Value, bool) {
	return nil, false
}

func (l FunctionPointer) Type() types.Type {
	if l.Fun == nil {
		return nil
	}
	return l.Fun.Type()
}

// Represents the nil pointer.
type NilLocation struct{}

func (n NilLocation) Hash() uint32 {
	return 42
}

func (n NilLocation) Equal(o Location) bool {
	_, ok := o.(NilLocation)
	return ok
}

func (n NilLocation) Position() string {
	return "<nil.Position>"
}

func (n NilLocation) String() string {
	return colorize.Nil("nil")
}

func (l NilLocation) GetSite() (ssa.Value, bool) {
	return nil, false
}

func (l NilLocation) Type() types.Type {
	return nil
}
