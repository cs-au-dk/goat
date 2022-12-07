package pkgutil_test

import (
	"log"
	"strings"
	"testing"

	"github.com/cs-au-dk/goat/analysis/upfront"
	p "github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func TestLoadWithModule(t *testing.T) {
	if pkgs, err := p.LoadPackages(p.LoadConfig{
		GoPath:     "../examples",
		ModulePath: "../examples/src/pkg-with-module",
	}, "unrelated-name/..."); err != nil {
		t.Fatal(err)
	} else if len(pkgs) != 2 {
		t.Errorf("Expected load result to contain 2 packages, got: %s", pkgs)
	}
}

func TestLoadFromGoPath(t *testing.T) {
	if pkgs, err := p.LoadPackages(p.LoadConfig{GoPath: "../examples"}, "pkg-with-test/..."); err != nil {
		t.Fatal(err)
	} else if len(pkgs) != 2 {
		t.Errorf("Expected load result to contain 2 packages, got: %s", pkgs)
	}
}

func TestLoadTestBloat(t *testing.T) {
	pkgs, err := p.LoadPackages(p.LoadConfig{
		IncludeTests: true,
		GoPath:       "../examples",
	}, "pkg-with-test-bloat/...")
	if err != nil {
		t.Fatal(err)
	} else if len(pkgs) != 3 {
		t.Fatalf("Expected 3 packages, got %d: %v", len(pkgs), pkgs)
	}

	prog, initPkgs := ssautil.AllPackages(pkgs, ssa.InstantiateGenerics)
	prog.Build()

	mains := ssautil.MainPackages(prog.AllPackages())

	if len(mains) == 0 {
		log.Println("No main packages detected")
		return
	}

	ptaResult := upfront.Andersen(prog, mains, upfront.IncludeType{All: true})

	ssaPkg := prog.ImportedPackage("pkg-with-test-bloat/sub")
	if ssaPkg == nil {
		t.Fatal("Cannot find sub-package?")
	}

	fun := ssaPkg.Func("Fun")
	if fun == nil {
		t.Fatal("Cannot find 'Fun' in sub-package")
	}

	i, ok := utils.FindSSAInstruction(fun, func(i ssa.Instruction) bool {
		_, ok := i.(*ssa.Call)
		return ok
	})
	if !ok {
		t.Fatal("Unable to find call-instruction in Fun")
	}

	call := i.(*ssa.Call)
	ptr := ptaResult.Queries[call.Call.Value].PointsTo()
	if !strings.Contains(ptr.String(), "(pkg-with-test-bloat.T).PublicMethod$1") {
		t.Fatalf("Expected %v to contain a function pointer to T.PublicMethod$1", ptr)
	}

	labels := ptr.Labels()
	switch len(labels) {
	case 1: // OK
		t.Log(labels)
	case 2:
		t.Error("Points-to sets are bloated from identical copies of packages:", pkgs)
		for _, l := range labels {
			v := l.Value()
			p := v.Parent().Pkg
			found := false
			for i, sp := range initPkgs {
				if p == sp {
					t.Logf("%v from %v %p %v", l, p, p, pkgs[i])
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("No package in initPkgs corresponds to %v (from %v)", p, l)
			}
		}
	default:
		t.Errorf("Unexpected number of labels (%d): %v", len(labels), labels)
	}
}
