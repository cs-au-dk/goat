package absint

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tu "github.com/cs-au-dk/goat/testutil"

	"golang.org/x/tools/go/ssa"
)

func runAllFunctionTests(t *testing.T, loadRes tu.LoadResult, fun absIntCommTestFunc) {
	for _, mp := range loadRes.Mains {
		for _, mem := range mp.Members {
			if f, ok := mem.(*ssa.Function); ok {
				if f != nil && f.Synthetic == "" && f.Name() != "main" &&
					f.Name() != "init" {
					if _, ok := loadRes.Cfg.Functions()[f]; ok {
						runTest(t, loadRes, fun, PrepareAI().Function(f))
					}
				}
			}
		}
	}
	// runTest(t, loadRes, fun, PrepareAI().FunctionByName("f", true))
}

func TestStaticAnalysisTopPointers(t *testing.T) {
	tests := tu.ListTopPointerTests(t, "../..", []string{})

	for _, test := range tests {
		tname := strings.SplitN(test, "/", 2)[1]

		t.Run(tname, func(t *testing.T) {
			fmt.Println("Starting:", test, "at", time.Now())
			runAllFunctionTests(t,
				tu.LoadExamplePackage(t, "../..", test),
				StaticAnalysisAndBlockingTests,
			)
			fmt.Println("Done: ", test, "at", time.Now())
		})
	}
}
