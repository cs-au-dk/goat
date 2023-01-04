package loopinline_test

import (
	"go/printer"
	"strings"
	"testing"

	"github.com/cs-au-dk/goat/analysis/absint"
	"github.com/cs-au-dk/goat/analysis/defs"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/analysis/upfront/loopinline"
	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/testutil"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestLoopInlineBenchmarks(t *testing.T) {
	t.Parallel()
	pathToRoot := "../../.."

	for _, test := range testutil.ListAllTests(t, pathToRoot) {
		test := test
		t.Run(test, func(t *testing.T) {
			t.Parallel()

			pkgs, err := pkgutil.LoadPackages(pkgutil.LoadConfig{GoPath: pathToRoot + "/examples"}, test)
			if err != nil {
				t.Fatal(err)
			}

			prt := func() {
				for _, pkg := range pkgs {
					var buf strings.Builder
					printer.Fprint(&buf, pkg.Fset, pkg.Syntax[0])
					t.Log(buf.String())
				}
			}

			if err := u.ASTTranform(pkgs, loopinline.Transform); err != nil {
				prt()
				t.Fatal("Inlining failed", err)
			}

			defer func() {
				if err := recover(); err != nil {
					prt()
					t.Fatal(err)
				}
			}()

			prog, _ := ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions|ssa.BuildSerially)
			prog.Build()
		})
	}
}

// Test that loop inlining has the desired effect of removing false positive
// reports in a specific communication pattern.
func TestLoopInlineAbsintEffect(t *testing.T) {
	program := `
	package main

	import "runtime"

	func doWork(i int) int {
		return i & -i
	}

	func main() {
		bound := runtime.GOMAXPROCS(0)

		ch := make(chan int)
		for i := 1; i <= bound; i++ {
			i := 0
			go func() {
				res := doWork(i)
				ch <- res
			}()
		}

		for i := 1; i <= bound; i++ {
			println(<-ch)
		}
	}`

	pkgs, err := pkgutil.LoadPackagesFromSource(program)
	if err != nil {
		t.Fatal("Failed to load packages", err)
	}

	for _, doInline := range []bool{false, true} {
		if doInline {
			err = u.ASTTranform(pkgs, loopinline.Transform)
			if err != nil {
				t.Fatal("Failed to perform loop inlining?", err)
			}
		}

		loadRes := testutil.LoadResultFromPackages(t, pkgs)
		C := absint.PrepareAI().WholeProgram(loadRes)
		S, result := absint.StaticAnalysis(C)
		bs := absint.BlockAnalysis(C, S, result)

		hasBlockingBugs := bs.Exists(func(_ defs.Superloc, _ map[defs.Goro]struct{}) bool {
			return true
		})

		if hasBlockingBugs == doInline {
			t.Errorf("hasBlockingBugs: %v, expected: %v\n%s", hasBlockingBugs, !doInline, bs.String())
		}
	}
}
