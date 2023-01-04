package upfront

import (
	"fmt"
	"go/types"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type (
	// TargetType is an alias for int.
	TargetType int

	// IncludeType can be used to configure the values of which types should be
	// included as queries in the points-to analysis.
	// Only pointer-like types appear in this list
	IncludeType struct {
		// All supersedes the other fields if set true.
		All bool

		// Pointer-like types
		Chan      bool
		Interface bool
		Function  bool
		Map       bool
		Slice     bool
		Pointer   bool
	}
)

// The types of points-to analysis targets, and the corresponding
// points-to query.
const (
	_NOT_TYPE TargetType = 1 << iota
	_DIRECT
	_INDIRECT
)

func (t TargetType) String() string {
	switch t {
	case _DIRECT:
		return "Direct"
	case _INDIRECT:
		return "Indirect"
	}
	return "?"
}

// collectPtsToQueries adds points-to queries to the PTA configuration based on the
// properties of the "include" value. It also computes the set of extended queries for
// the Locker fields of Cond primitives.
func collectPtsToQueries(
	prog *ssa.Program,
	config *pointer.Config,
	include IncludeType,
) map[ssa.Value]*pointer.Pointer {
	cQueries := map[ssa.Value]*pointer.Pointer{}
	maybeAdd := func(v ssa.Value) {
		prettyPrint := func(v ssa.Value) {
			opts.OnVerbose(func() {
				pos := prog.Fset.Position(v.Pos())
				fmt.Printf("%d corresponds to: %s\n", v.Pos(), pos)
				fmt.Printf("ssa.Value type: %s, underlying type: %s\n\n",
					v.Type().String(), v.Type().Underlying().String())
			})
		}

		if typ := include.checkType(v.Type()); typ != _NOT_TYPE {
			prettyPrint(v)
			if typ&_DIRECT != 0 {
				config.AddQuery(v)

				// Add extended queries for the Locker field of Cond objects.
				// TODO: This assumes that Cond objects are always allocated directly (not
				// embedded in another struct), which I guess is not a safe assumption?
				if pt, ok := v.Type().Underlying().(*types.Pointer); ok && !opts.SkipSync() {
					if _, isAlloc := v.(*ssa.Alloc); isAlloc &&
						utils.IsNamedTypeStrict(pt.Elem(), "sync", "Cond") {

						ptr, err := config.AddExtendedQuery(v, "x.L")
						if err != nil {
							log.Fatalf("Failed to add extended query: %v", err)
						}

						cQueries[v] = ptr
					}
				}
			}
			if typ&_INDIRECT != 0 {
				config.AddIndirectQuery(v)
			}
		}
	}

	for fun := range ssautil.AllFunctions(prog) {
		verbosePrint("Collecting channels and functions in: %s\n", fun.Name())
		for _, param := range fun.Params {
			maybeAdd(param)
		}
		for _, fv := range fun.FreeVars {
			maybeAdd(fv)
		}

		for _, block := range fun.Blocks {
			for _, insn := range block.Instrs {
				switch v := insn.(type) {
				case *ssa.Call:
					common := v.Common()
					// if common.IsInvoke() {
					maybeAdd(common.Value)
					// }
					maybeAdd(v)
				case *ssa.Range:
				case ssa.Value:
					maybeAdd(v)
				}
			}
		}
		verbosePrint("\n")
	}

	return cQueries
}

// checkType ensures that the given type should be targetted
func (include IncludeType) checkType(t types.Type) TargetType {
	switch t := t.(type) {
	case *types.Named:
		return include.checkType(t.Underlying())
	case *types.Chan:
		if include.All || include.Chan {
			return _DIRECT
		}
	case *types.Signature:
		if include.All || include.Function {
			return _DIRECT
		}
	case *types.Interface:
		if include.All || include.Interface {
			return _DIRECT
		}
	case *types.Map:
		if include.All || include.Map {
			return _DIRECT
		}
	case *types.Slice:
		if include.All || include.Slice {
			return _DIRECT
		}
	case *types.Pointer:
		// If pointers are not considered for the PTA, add indirect queues
		// for the types which are included
		var res = _NOT_TYPE
		if include.checkType(t.Elem()) != _NOT_TYPE {
			res = _INDIRECT
		}
		// If pointers are considered for the PTA, then also add them directly
		if include.All || include.Pointer {
			return res + _DIRECT
		}
	}

	return _NOT_TYPE
}

// GetPtsToSets returns the points-to results for channels, functions and interfaces.
func GetPtsToSets(prog *ssa.Program, mains []*ssa.Package) *PointerResult {
	return Andersen(prog, mains, IncludeType{
		Chan:      true,
		Function:  true,
		Interface: true,
	})
}

type PointerResult struct {
	pointer.Result
	CondQueries map[ssa.Value]*pointer.Pointer
}

// Andersen is a wrapper around the points-to analysis. Requires a program, a list of main packages
// and an include configuration according to which points-to queries may be collected.
func Andersen(prog *ssa.Program, mains []*ssa.Package, include IncludeType) *PointerResult {
	a_config := &pointer.Config{
		Mains:          mains,
		BuildCallGraph: true,
	}

	cQueries := collectPtsToQueries(prog, a_config, include)

	result, err := pointer.Analyze(a_config)
	if err != nil {
		fmt.Println("Failed pointer analysis")
		fmt.Println(err)
		os.Exit(1)
	}

	return &PointerResult{*result, cQueries}
}

// TotalAndersen performs points-to analysis for all pointer-like values in the given SSA program.
func TotalAndersen(prog *ssa.Program, mains []*ssa.Package) *PointerResult {
	return Andersen(prog, mains, IncludeType{
		All: true,
	})
}

type (
	// accessPath is a baseline from which access actions used by points-to labels may be derived.
	accessPath struct{}

	// FieldAccess is an access action that models reading the field of a struct value. The "Field"
	// field encodes the name of the field.
	FieldAccess struct {
		accessPath
		Field string
	}
	// ArrayAccess is an access action that models reading an index in an array or slice value.
	ArrayAccess struct{ accessPath }

	// Acccess is implemented by all access actions.
	Access interface{ accessTag() }
)

// accessTag is implemented by any access action
func (accessPath) accessTag() {}

// Access paths can be of the form: x.y.[*].
// We specify that field names can contain anything but a dot and an open square bracket
var pathRegexp = regexp.MustCompile(`\.[^.[]+|\[\*\]`)

// SplitLabel takes a points-to analysis label and splits it into the root SSA value,
// and a sequence of access actions composing an access path.
func SplitLabel(label *pointer.Label) (ssa.Value, []Access) {
	v := label.Value()
	if path := label.Path(); path == "" {
		// If the label does not contain an access path, return the SSA value
		// and an empty set of access actions.
		return v, nil
	} else {
		components := pathRegexp.FindAllString(path, -1)
		if strings.Join(components, "") != path {
			log.Fatalln("Path match was not full", components, path, label)
		}

		accesses := make([]Access, len(components))
		for i, f := range components {
			// Check whether the access action is an array access, [*], or field access.
			if f == "[*]" {
				accesses[i] = ArrayAccess{}
			} else {
				// The dot before the field name is discarded
				accesses[i] = FieldAccess{Field: f[1:]}
			}
		}
		return v, accesses
	}
}
