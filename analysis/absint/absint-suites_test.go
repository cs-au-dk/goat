package absint

import (
	"strings"
	"testing"

	tu "github.com/cs-au-dk/goat/testutil"
)

func TestBenchmarkSuites(t *testing.T) {
	suites := []struct {
		name     string
		tests    []string
		namefunc func(string) string
	}{
		{
			"Simple",
			tu.ListSimpleTests(t, "../.."),
			func(tname string) string { return strings.TrimPrefix(tname, "simple-examples/") },
		},
		{
			"SessionTypes",
			// Blacklist powsers as it "takes too bloody long"
			tu.ListSessionTypesPackages(t, "../..", []string{"powsers"}),
			func(tname string) string { return strings.Split(tname, "/")[1] },
		},
		{
			"SyncPkg",
			tu.ListSyncPkgTests(t, "../..", nil),
			func(tname string) string { return strings.SplitN(tname, "/", 2)[1] },
		},
		{
			"SyncPkgCond",
			tu.ListSyncPkgCondTests(t, "../..", nil),
			func(tname string) string { return strings.Split(tname, "/")[1] },
		},
		{
			"GoKer",
			func() []string {
				if testing.Short() {
					t.Log("Skipping GoKer tests in -short mode")
					return nil
				}

				tests := tu.ListGoKerPackages(t, "../..")

				// NOTE: The same functionality can be achieved by specifying the -run parameter
				// on the command line. E.g.:
				// go test github.com/cs-au-dk/goat/analysis/cegar -run TestStaticAnalysisGoKer/moby/4395
				included := func(test string) bool {
					// Change to false to focus only on specific GoKer tests
					const (
						VAR = true
						// VAR = false
					)
					if VAR {
						return true
					}

					tests := []string{
						// "cockroach/584",
						// "cockroach/584_fixed",
						// "cockroach/2448",
						// "cockroach/2448_fixed",
						// "cockroach/3710",
						// "cockroach/6181",
						// "cockroach/7504",
						// "cockroach/9935",
						// "cockroach/10214",
						// "cockroach/10790",
						// "cockroach/13197",
						// "cockroach/13755",
						// "cockroach/16167",
						// "cockroach/18101",
						// "cockroach/24808",
						// "cockroach/24808_fixed",
						// "cockroach/25456",
						// "cockroach/35073",
						// "cockroach/25456_fixed",
						// "cockroach/35931",
						// "cockroach/35931_fixed",
						// "etcd/5509",
						// "etcd/5509_fixed",
						// "etcd/6708",
						// "etcd/6857",
						"etcd/7443",
						// "etcd/7492",
						// "etcd/7902",
						// "etcd/10492",
						// "etcd/6857",
						// "etcd/6873",
						// "etcd/6873_fixed",
						// "grpc/660",
						// "grpc/795",
						// "grpc/795_fixed",
						// "grpc/862",
						// "grpc/1275",
						// "grpc/1353",
						// "grpc/1460",
						// "hugo/5379",
						// "istio/16224"
						// "istio/17860",
						// "istio/18454",
						// "kubernetes/1321",
						// "kubernetes/5316",
						// "kubernetes/5316_fixed",
						// "kubernetes/6632",
						// "kubernetes/6632_fixed",
						// "kubernetes/10182",
						// "kubernetes/11298", // FIXME: Check
						// "kubernetes/13135",
						// "kubernetes/26980", // FIXME: Check
						// "kubernetes/30872",
						// "kubernetes/38669",
						// "kubernetes/58107",
						// "kubernetes/62464",
						// "kubernetes/70277",
						// "moby/4395",
						// "moby/4395_fixed",
						// "moby/4951",
						// "moby/7559",
						// "moby/17176",
						// "moby/21233",
						// "moby/27782",
						// "moby/28462",
						// "moby/28462_fixed",
						// "moby/29733",
						// "moby/30408",
						// "moby/33781", // FIXME: Check
						// "moby/36114",
						// "serving/2137",
						// "syncthing/4829",
						// "syncthing/5795",
					}

					for _, t := range tests {
						if t == tu.GoKerTestName(test) {
							return true
						}
					}

					return false

				}

				filtered := make([]string, 0, len(tests))
				for _, test := range tests {
					if included(test) {
						filtered = append(filtered, test)
					}
				}

				return filtered
			}(),
			tu.GoKerTestName,
		},
	}

	for _, suite := range suites {
		t.Run(suite.name, func(t *testing.T) {
			for _, test := range suite.tests {
				tname := suite.namefunc(test)

				t.Run(tname, func(t *testing.T) {
					test := test
					t.Parallel()
					tu.ParallelHelper(t,
						tu.LoadExampleAsPackages(t, "../..", test, false),
						func(loadRes tu.LoadResult) {
							runWholeProgTest(t, loadRes, StaticAnalysisAndBlockingTests)
							runFocusedPrimitiveTests(t, loadRes, nil)
						},
					)
				})
			}
		})
	}
}
