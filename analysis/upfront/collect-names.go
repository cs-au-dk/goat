package upfront

import (
	"fmt"
	"go/ast"
	"go/token"
	"log"
	"sync"
	"time"

	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/packages"
)

// chanNameCollector is a visitor for collecting channel names in the AST
// and mapping them to positions, such that SSA `make(chan)` instructions
// can be mapped back to their corresponding names.
type chanNameCollector struct {
	sync.Mutex
	function *ast.FuncDecl
}

// ChannelNames is a map from source positions to strings denoting channel names.
var ChannelNames = make(map[token.Pos]string)

func (v *chanNameCollector) addName(pos token.Pos, name string) {
	funName := "<global>"
	if v.function != nil {
		funName = v.function.Name.Name
	}

	v.Lock()
	defer v.Unlock()
	ChannelNames[pos] = fmt.Sprintf("%s.%s", funName, name)
}

func (v *chanNameCollector) Visit(n ast.Node) ast.Visitor {
	switch s := n.(type) {
	case *ast.FuncDecl:
		// Update enclosing function for children
		return &chanNameCollector{function: s}

	case *ast.AssignStmt:
		for i, name := range s.Lhs {
			switch e1 := name.(type) {
			case *ast.Ident:
				if len(s.Rhs) <= i {
					continue
				}

				switch e2 := s.Rhs[i].(type) {
				case *ast.CallExpr:
					switch fun := e2.Fun.(type) {
					case *ast.Ident:
						if fun.Name == "make" {
							v.addName(e2.Lparen, e1.Name)
						}
					}
				}
			}
		}

	case *ast.ValueSpec:
		for i, ident := range s.Names {
			if len(s.Values) <= i {
				continue
			}

			switch e := s.Values[i].(type) {
			case *ast.CallExpr:
				switch fun := e.Fun.(type) {
				case *ast.Ident:
					if fun.Name == "make" {
						v.addName(e.Lparen, ident.Name)
					}
				}
			}
		}
	}
	return v
}

// CollectNames maps token position to channel names at the AST level.
func CollectNames(pkgs []*packages.Package) {
	if opts.JustGoros() {
		return
	}

	if opts.Verbose() {
		defer utils.TimeTrack(time.Now(), fmt.Sprintf("Collect channel names"))
	}

	// Reset channel name map
	ChannelNames = make(map[token.Pos]string)

	var wg sync.WaitGroup

	count := 0
	visitor := &chanNameCollector{}
	for _, pkg := range pkgs {
		wg.Add(len(pkg.Syntax))
		for _, file := range pkg.Syntax {
			go func(file *ast.File) {
				defer wg.Done()
				count++
				ast.Walk(visitor, file)
			}(file)
		}
	}

	wg.Wait()
	opts.OnVerbose(func() { log.Println("Collected names in", count, "files") })
}
