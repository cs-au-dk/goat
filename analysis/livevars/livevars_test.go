package livevars

import (
	"fmt"
	"runtime/debug"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/pkgutil"

	"testing"
	"time"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// Tests whether the livevars analysis finishes within 30 seconds for the "simple" example.
// The simple example loads the fmt package which in turn loads many other packages.
// This makes the program CFG reasonably large.
func TestLivevarsPerformance(t *testing.T) {
	// Create a main-package from the specified package
	pkgs, err := pkgutil.LoadPackages(pkgutil.LoadConfig{GoPath: "../../examples"}, "simple")
	if err != nil {
		t.Fatal(err)
	}

	upfront.CollectNames(pkgs)

	prog, _ := ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions)
	prog.Build()

	mains := ssautil.MainPackages(prog.AllPackages())

	pkgutil.GetLocalPackages(mains, prog.AllPackages())

	ptrinfo := upfront.GetPtsToSets(prog, mains)

	prog_cfg := cfg.GetCFG(prog, mains, ptrinfo)

	errCh := make(chan interface{}, 1)

	// Spawn a goroutine to do the actual live vars analysis.
	// TODO: When the test fails this goroutine is left running.
	// A solution could be to add a (cancel-)context parameter to the LiveVars analysis.
	go func() {
		defer func() {
			if err := recover(); err != nil {
				errCh <- fmt.Errorf("%s\n%s\n", err, debug.Stack())
			}
		}()

		LiveVars(*prog_cfg, ptrinfo)
		errCh <- nil
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Error(err)
		}
	case <-time.After(30 * time.Second):
		t.Error("Live variables analysis did not finish within 30 seconds.")
	}
}
