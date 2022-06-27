package upfront

import (
	"fmt"
	"go/types"
	"log"
	"os"
	"regexp"
	"strings"

	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

type TargetType int

// Configuration structure determines for values of which types
// to include queries in the Andersen analysis.
// Only pointer-like values appear in this list
type IncludeType struct {
	All       bool
	Chan      bool
	Interface bool
	Function  bool
	Map       bool
	// Array     bool
	Slice   bool
	Pointer bool
}

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

func collectPtsToQueries(prog *ssa.Program, config *pointer.Config, include IncludeType) {
	maybeAdd := func(v ssa.Value) {
		prettyPrint := func(v ssa.Value) {
			opts.OnVerbose(func() {
				pos := prog.Fset.Position(v.Pos())
				fmt.Printf("%d corresponds to: %s\n", v.Pos(), pos)
				fmt.Printf("ssa.Value type: %s, underlying type: %s\n\n",
					v.Type().String(), v.Type().Underlying().String())
			})
		}

		if typ := checkType(v.Type(), include); typ != _NOT_TYPE {
			prettyPrint(v)
			if typ&_DIRECT != 0 {
				config.AddQuery(v)
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
}

func checkType(t types.Type, include IncludeType) TargetType {
	switch t := t.(type) {
	case *types.Named:
		return checkType(t.Underlying(), include)
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
	// case *types.Array:
	// 	if include.All || include.Array {
	// 		return _DIRECT
	// 	}
	case *types.Slice:
		if include.All || include.Slice {
			return _DIRECT
		}
	case *types.Pointer:
		// If pointers are not considered for the PTA, add indirect queues
		// for the types which are included
		var res = _NOT_TYPE
		if checkType(t.Elem(), include) != _NOT_TYPE {
			res = _INDIRECT
		}
		// If pointers are considered for the PTA, then also add them directly
		if include.All || include.Pointer {
			return res + _DIRECT
		}
	}

	return _NOT_TYPE
}

func GetPtsToSets(prog *ssa.Program, mains []*ssa.Package) *pointer.Result {
	return Andersen(prog, mains, IncludeType{
		Chan:      true,
		Function:  true,
		Interface: true,
	})
}

func Andersen(prog *ssa.Program, mains []*ssa.Package, include IncludeType) *pointer.Result {
	a_config := &pointer.Config{
		Mains:          mains,
		BuildCallGraph: true,
	}

	collectPtsToQueries(prog, a_config, include)

	result, err := pointer.Analyze(a_config)
	if err != nil {
		fmt.Println("Failed pointer analysis")
		fmt.Println(err)
		os.Exit(1)
	}

	return result
}

func TotalAndersen(prog *ssa.Program, mains []*ssa.Package) *pointer.Result {
	return Andersen(prog, mains, IncludeType{
		All: true,
	})
}

type atag struct{}

func (atag) accessTag() {}

type FieldAccess struct {
	atag
	Field string
}
type ArrayAccess struct{ atag }

type Access interface{ accessTag() }

// We specify that field names can contain anything but a dot and an open square bracket
var pathRegexp = regexp.MustCompile(`\.[^.[]+|\[\*\]`)

func SplitLabel(label *pointer.Label) (ssa.Value, []Access) {
	v := label.Value()
	if path := label.Path(); path == "" {
		return v, nil
	} else {
		components := pathRegexp.FindAllString(path, -1)
		if strings.Join(components, "") != path {
			log.Fatalln("Path match was not full", components, path, label)
		}

		accesses := make([]Access, len(components))
		for i, f := range components {
			if f == "[*]" {
				accesses[i] = ArrayAccess{}
			} else {
				// The dot before the field name is discarded
				accesses[i] = FieldAccess{ Field: f[1:] }
			}
		}
		return v, accesses
	}
}
