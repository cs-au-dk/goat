package absint

import (
	"strings"
	"testing"

	"Goat/testutil"
)

func TestSessionTypesBenchmarks(t *testing.T) {
	blacklist := []string{
		"powsers", // Takes too bloody long
	}

	tests := testutil.ListSessionTypesPackages(t, "../..", blacklist)

	for _, test := range tests {
		tname := strings.Split(test, "/")[1]

		t.Run(tname, func(t *testing.T) {
			loadRes := testutil.LoadExamplePackage(t, "../..", test)
			runWholeProgTest(t, loadRes, StaticAnalysisAndBlockingTests)
			runFocusedPrimitiveTests(t, loadRes)
		})
	}
}
