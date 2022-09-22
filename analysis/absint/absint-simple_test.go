package absint

import (
	"strings"
	"testing"

	"github.com/cs-au-dk/goat/testutil"
)

func TestSimpleBenchmarks(t *testing.T) {
	tests := testutil.ListSimpleTests(t, "../..")

	for _, test := range tests {
		tname := strings.TrimPrefix(test, "simple-examples/")

		t.Run(tname, func(t *testing.T) {
			loadRes := testutil.LoadExamplePackage(t, "../..", test)
			runWholeProgTest(t, loadRes, StaticAnalysisAndBlockingTests)
			runFocusedPrimitiveTests(t, loadRes)
		})
	}
}
