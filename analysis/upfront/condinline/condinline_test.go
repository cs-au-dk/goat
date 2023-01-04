package condinline_test

import (
	"github.com/cs-au-dk/goat/analysis/gotopo"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/analysis/upfront/condinline"
	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/testutil"
	"github.com/cs-au-dk/goat/utils"
	"go/printer"
	"strings"
	"testing"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestCondInline(t *testing.T) {
	testProg := `
	package main

	import "sync"

	func main() {
		var mu sync.Mutex
		cond := sync.NewCond(&mu)
		cond.Signal()
	}`

	pkgs, err := pkgutil.LoadPackagesFromSource(testProg)
	if err != nil {
		t.Fatal("Failed to load package:", err)
	}

	err = u.ASTTranform(pkgs, condinline.Transform)
	if err != nil {
		t.Errorf("Error occurred during inlining %v", err)
	}

	var buf strings.Builder
	pkg := pkgs[0]
	printer.Fprint(&buf, pkg.Fset, pkg.Syntax[0])
	pp := buf.String()

	expected := `package main

import "sync"

func main() {
	var mu sync.Mutex
	cond := &sync.Cond{L: &mu}
	cond.Signal()
}
`

	if pp != expected {
		t.Errorf("Expected:\n%q\nActual:\n%q", expected, pp)
	}

	// Test that the package builds
	prog, _ := ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions|ssa.BuildSerially)
	prog.Build()
}

func TestCondInlineSeparatesConds(t *testing.T) {
	testProg := `
	package main

	import "sync"

	func main() {
		var mu sync.Mutex
		cond1 := sync.NewCond(&mu)
		cond1.Signal()
		cond2 := sync.NewCond(&mu)
		cond2.Signal()
	}`

	loadRes := testutil.LoadPackageFromSource(t, "test", testProg)
	mainPkg := loadRes.Mains[0]
	G := loadRes.PrunedCallDAG.Original

	mainFun := mainPkg.Func("main")
	_, primsToUses := gotopo.GetPrimitives(mainFun, loadRes.Pointer, G, false)
	if len(primsToUses) != 2 {
		t.Errorf("Expected exactly 2 primitives uses, got: %v", primsToUses)
	}

	for prim, uses := range primsToUses {
		if !utils.IsNamedType(prim.Type(), "sync", "Cond") {
			t.Errorf("Expected prim to be a sync.Cond, was: %v", prim.Type())
		} else if _, found := uses[mainFun]; !found {
			t.Errorf("Expected cond to be used in main, got: %v", uses)
		}
	}
}
