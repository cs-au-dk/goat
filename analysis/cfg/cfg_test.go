package cfg

import (
	"Goat/analysis/upfront"
	"Goat/pkgutil"
	"Goat/utils/worklist"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestCompressAndPanicCont(t *testing.T) {
	prog := `package main
	func f() {}
	func main() {
		ch := make(chan int)
		defer close(ch)
		f()
	}`

	pkgs, err := pkgutil.LoadPackagesFromSource(prog)
	if err != nil {
		t.Fatal("Failed to load program:", err)
	}

	program, ssaPkgs := ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions)
	program.Build()
	results := upfront.Andersen(program, ssaPkgs, upfront.IncludeType{All: true})

	cfg.init()
	cfg.fset = program.Fset

	mainFun := ssaPkgs[0].Func("main")

	io := getFunCfg(program, mainFun, results)
	cfg.addEntry(io.in)

	// The built-in traversal functions seem to consider the DeferLink, which
	// leads from function exit to entry, which is undesirable.
	find := func(from Node, pred func(Node) bool, msg string) Node {
		visited := map[Node]bool{from: true}
		Q := worklist.Empty[Node]()
		Q.Add(from)

		for !Q.IsEmpty() {
			node := Q.GetNext()
			if pred(node) {
				return node
			}

			for succ := range node.Successors() {
				if !visited[succ] {
					visited[succ] = true
					Q.Add(succ)
				}
			}
		}

		t.Fatal("Failed to find node:", msg)
		panic("unreachable")
	}

	postcall := find(cfg.funs[mainFun].entry, func(node Node) bool {
		if pcall, ok := node.(*PostCall); ok {
			if ssaNode, ok := pcall.CallRelationNode().(*SSANode); ok {
				return ssaNode.Instruction().(*ssa.Call).Call.StaticCallee().Name() == "f"
			} else {
				return false
			}
		} else {
			return false
		}
	}, "postcall")

	panicToClose := func(msg string) {
		dfr := postcall.PanicCont()
		if dfr == nil {
			t.Fatal("Postcall node has no PanicCont?")
		}

		find(dfr, func(node Node) bool {
			if synth, ok := node.(*BuiltinCall); ok {
				_ = synth
				return true
			} else {
				return false
			}
		}, msg)
	}

	//VisualizeFunction(mainFun)

	panicToClose("close-beforeCompress")

	compressCfg()

	//VisualizeFunction(mainFun)

	panicToClose("close-afterCompress")
}
