package absint

import (
	"fmt"
	"go/types"
	"log"
	"sort"
	"strings"
	"testing"
	"time"

	"Goat/analysis/defs"
	"Goat/analysis/gotopo"
	L "Goat/analysis/lattice"
	"Goat/pkgutil"
	"Goat/testutil"
	tu "Goat/testutil"

	"github.com/fatih/color"
	"golang.org/x/tools/go/ssa"
)

func StaticAnalysisAndBlockingTests(t *testing.T, C AnalysisCtxt, a L.Analysis, sg SuperlocGraph, nm testutil.NotesManager) {
	ChannelValueQueryTests(t, C, a, sg, nm)
	BlockAnalysisTest(t, C, a, sg, nm)
}

func TestStaticAnalysisGoKer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping test in -short mode")
	}

	tests := testutil.ListGoKerPackages(t, "../..")

	// NOTE: The same functionality can be achieved by specifying the -run parameter
	// on the command line. E.g.:
	// go test Goat/analysis/cegar -run TestStaticAnalysisGoKer/moby/4395
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
			if t == testutil.GoKerTestName(test) {
				return true
			}
		}

		return false

	}

	metrics = blockAnalysisMetrics{}
	for _, test := range tests {
		if !included(test) {
			continue
		}

		t.Run(testutil.GoKerTestName(test), func(t *testing.T) {
			fmt.Println("Starting:", test, "at", time.Now())
			loadRes := testutil.LoadExamplePackage(t, "../..", test)
			runWholeProgTest(t, loadRes, StaticAnalysisAndBlockingTests)
			fmt.Println("Done: ", test, "at", time.Now())

			runFocusedPrimitiveTests(t, loadRes)
		})
	}

	t.Logf("%+v", metrics)
}

func TestGoKerLocalized(t *testing.T) {
	// rg "Communication Deadlock \| Channel" examples/src/ --files-with-matches | sed 's|/README.md||g' | sort
	// Channel & Condition Variable manually removed
	tests := strings.Split(`
gobench/goker/blocking/cockroach/10790
gobench/goker/blocking/cockroach/13197
gobench/goker/blocking/cockroach/13755
gobench/goker/blocking/cockroach/2448
gobench/goker/blocking/cockroach/24808
gobench/goker/blocking/cockroach/25456
gobench/goker/blocking/cockroach/35073
gobench/goker/blocking/cockroach/35931
gobench/goker/blocking/etcd/6857
gobench/goker/blocking/grpc/1275
gobench/goker/blocking/grpc/1424
gobench/goker/blocking/grpc/660
gobench/goker/blocking/grpc/862
gobench/goker/blocking/istio/17860
gobench/goker/blocking/istio/18454
gobench/goker/blocking/kubernetes/25331
gobench/goker/blocking/kubernetes/38669
gobench/goker/blocking/kubernetes/5316
gobench/goker/blocking/kubernetes/70277
gobench/goker/blocking/moby/21233
gobench/goker/blocking/moby/33293
gobench/goker/blocking/moby/33781
gobench/goker/blocking/moby/4395
gobench/goker/blocking/syncthing/5795`, "\n")[1:]

	metrics = blockAnalysisMetrics{}
	for _, test := range tu.ListGoKerPackages(t, "../..") {
		included := false
		for _, curated := range tests {
			if strings.HasPrefix(test, curated) {
				included = true
				break
			}
		}

		if !included {
			continue
		}

		t.Run(testutil.GoKerTestName(test), func(t *testing.T) {
			loadRes := testutil.LoadExamplePackage(t, "../..", test)
			entry := loadRes.Mains[0].Func("main")
			if entry == nil {
				t.Fatal("Could not find main function on", loadRes.Mains[0])
			}

			callDAG := loadRes.CallDAG
			G := callDAG.Original

			computeDominator := G.DominatorTree(entry)

			ps := gotopo.GetPrimitives(entry, loadRes.Pointer, G)
			// Filter out non-local primitives and primitives whose allocation
			// site are not reachable in the call graph.
			// Currently also filters out non-channel primitives.
			for _, usageInfo := range ps {
				for _, usedPrimitives := range []map[ssa.Value]struct{}{
					usageInfo.Chans(),
					usageInfo.OutChans(),
					// usageInfo.Sync(),
				} {
					for prim := range usedPrimitives {
						if _, isCh := prim.Type().Underlying().(*types.Chan); !isCh ||
							prim.Parent() == nil || !pkgutil.IsLocal(prim) ||
							callDAG.ComponentOf(prim.Parent()) == -1 {
							delete(usedPrimitives, prim)
						}
					}
				}
			}

			// GCatch PSets
			psets := gotopo.GetGCatchPSets(
				loadRes.Cfg, entry, loadRes.Pointer, G,
				computeDominator, callDAG, ps,
			)

			// Ensure consistent ordering
			sort.Slice(psets, func(i, j int) bool {
				return psets[i].String() < psets[j].String()
			})

			// Compute map from primitives to functions in which they are used
			primsToUses := map[ssa.Value]map[*ssa.Function]struct{}{}
			for fun, usageInfo := range ps {
				if callDAG.ComponentOf(fun) == -1 {
					continue
				}

				for _, usedPrimitives := range []map[ssa.Value]struct{}{
					usageInfo.Chans(),
					usageInfo.OutChans(),
					// TODO: Temporarily disabled because analysis of locks
					// seem to time out more often than analysis of channels.
					//usageInfo.Sync(),
				} {
					for prim := range usedPrimitives {
						if _, seen := primsToUses[prim]; !seen {
							primsToUses[prim] = make(map[*ssa.Function]struct{})
						}
						primsToUses[prim][fun] = struct{}{}
					}
				}
			}

			blocks := make(Blocks)

			for i, pset := range psets {
				t.Log(color.CyanString("Found PSet"), i+1, color.CyanString("of"), len(psets), color.CyanString(":"))
				t.Log(pset, "\n")

				funs := []*ssa.Function{}
				pset.ForEach(func(v ssa.Value) {
					if v.Parent() != nil {
						// Include allocation site in dominator computation
						funs = append(funs, v.Parent())
					}
					for fun := range primsToUses[v] {
						funs = append(funs, fun)
					}
				})

				loweredEntry := computeDominator(funs...)
				t.Log("Using", loweredEntry, "as entrypoint")

				C := ConfigAI(AIConfig{Metrics: true}).Function(loweredEntry)(loadRes)
				C.FragmentPredicateFromPrimitives(pset.Entries(), primsToUses)

				C.Metrics.TimerStart()
				ts, analysis := StaticAnalysis(C)

				//log.Println("Superlocation graph size:", ts.Size())

				switch C.Metrics.Outcome {
				case OUTCOME_PANIC:
					log.Println(color.RedString("Aborted!"))
					log.Println(C.Metrics.Error())
				default:
					C.Metrics.Done()
					log.Println(color.GreenString("SA completed in %s", C.Metrics.Performance()))

					blocks.UpdateWith(BlockAnalysis(C, ts, analysis))
				}
			}

			findClInSl := func(ann tu.AnnProgress) func(defs.Goro, defs.CtrLoc) bool {
				return func(g defs.Goro, cl defs.CtrLoc) bool {
					if !ann.HasFocus() || ann.Focused().Matches(g) {
						for node := range ann.Nodes() {
							if cl.Node() == node {
								return true
							}
						}
					}
					return false
				}
			}

			findCl := func(ann tu.AnnProgress) func(sl defs.Superloc, gs map[defs.Goro]struct{}) bool {
				inner := findClInSl(ann)
				return func(sl defs.Superloc, gs map[defs.Goro]struct{}) bool {
					for g := range gs {
						if inner(g, sl.GetUnsafe(g)) {
							return true
						}
					}
					return false
				}
			}

			testutil.MakeNotesManager(t, loadRes).ForEachAnnotation(func(a tu.Annotation) {
				if ann, ok := a.(tu.AnnBlocks); ok {
					isChOp := false
					for node := range ann.Nodes() {
						if node.IsChannelOp() {
							isChOp = true
							break
						}
					}

					if !isChOp {
						return
					}

					if !blocks.Exists(findCl(ann)) {
						t.Log("False negative:", ann)
						metrics.falseNegatives++
					} else {
						metrics.truePositives++
					}
				}
			})
		})
	}

	t.Log(metrics)
}
