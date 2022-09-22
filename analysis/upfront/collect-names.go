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

type ChanNameCollector struct {
	function *ast.FuncDecl
}

var (
	lock         = make(chan bool, 1)
	ChannelNames = make(map[token.Pos]string)
)

func (v *ChanNameCollector) addName(pos token.Pos, name string) {
	funName := "<global>"
	if v.function != nil {
		funName = v.function.Name.Name
	}

	lock <- true
	ChannelNames[pos] = fmt.Sprintf("%s.%s", funName, name)
	<-lock
}

func (v *ChanNameCollector) Visit(n ast.Node) ast.Visitor {
	switch s := n.(type) {
	case *ast.FuncDecl:
		// Update enclosing function for children
		return &ChanNameCollector{function: s}

	case *ast.AssignStmt:
		for i, name := range s.Lhs {
			switch e1 := name.(type) {
			case *ast.Ident:
				if len(s.Rhs) > i {
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
		}

	case *ast.ValueSpec:
		for i, ident := range s.Names {
			if len(s.Values) > i {
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
	}
	return v
}

func CollectNames(pkgs []*packages.Package) {
	if !opts.JustGoros() {
		if opts.Verbose() {
			defer utils.TimeTrack(time.Now(), fmt.Sprintf("Collect channel names"))
		}

		// Reset channel name map
		ChannelNames = make(map[token.Pos]string)

		var wg sync.WaitGroup

		count := 0
		visitor := new(ChanNameCollector)
		for _, pkg := range pkgs {
			for _, file := range pkg.Syntax {
				wg.Add(1)
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
}
