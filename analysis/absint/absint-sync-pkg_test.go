package absint

import (
	"fmt"
	"strings"
	"testing"

	"Goat/testutil"
)

func TestStaticAnalysisSyncPkg(t *testing.T) {
	blacklist := []string{}

	tests := testutil.ListSyncPkgTests(t, "../..", blacklist)

	for _, test := range tests {
		tname := strings.SplitN(test, "/", 2)[1]

		t.Run(tname, func(t *testing.T) {
			loadRes := testutil.LoadExamplePackage(t, "../..", test)
			runWholeProgTest(t, loadRes, StaticAnalysisAndBlockingTests)
			fmt.Println("Done: ", test)
			runFocusedPrimitiveTests(t, loadRes)
		})
	}
}
