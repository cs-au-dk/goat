package absint

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/cs-au-dk/goat/analysis/defs"
	"github.com/cs-au-dk/goat/analysis/gotopo"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	tu "github.com/cs-au-dk/goat/testutil"

	"github.com/fatih/color"
	"github.com/sebdah/goldie/v2"
	"golang.org/x/tools/go/ssa"
)

func StaticAnalysisAndBlockingTests(t *testing.T, C AnalysisCtxt, a L.Analysis, sg SuperlocGraph, nm tu.NotesManager) {
	ChannelValueQueryTests(t, C, a, sg, nm)
	BlockAnalysisTest(t, C, a, sg, nm)
}

func TestGoKerLocalized(t *testing.T) {
	// rg "Communication Deadlock \| Channel \|" examples/src/ --files-with-matches | sed 's|/README.md||g' | sort
	// gobench/goker/blocking/cockroach/25456, gobench/goker/blocking/cockroach/35931
	// - wildcard swap due to getting a struct with the channel outside the fragment
	// gobench/goker/blocking/cockroach/35073 - needs loop unrolling and context sensitivity
	// gobench/goker/blocking/grpc/660 - spawn in infinite loop
	// gobench/goker/blocking/kubernetes/38669 - not the correct PSet, interprocedural deps only
	// gobench/goker/blocking/moby/21233 - wildcard swap due to passing channel out of fragment and back
	// gobench/goker/blocking/moby/33781 - spawn in infinite loop
	// gobench/goker/blocking/syncthing/5795 - wildcard swap due to storing closure on top object
	tests := strings.Split(`
gobench/goker/blocking/cockroach/2448
gobench/goker/blocking/cockroach/24808
gobench/goker/blocking/etcd/6857
gobench/goker/blocking/grpc/1275
gobench/goker/blocking/grpc/1424
gobench/goker/blocking/istio/17860
gobench/goker/blocking/kubernetes/5316
gobench/goker/blocking/kubernetes/70277
gobench/goker/blocking/moby/33293
gobench/goker/blocking/moby/4395`, "\n")[1:]

	metrics = blockAnalysisMetrics{}
	for _, test := range tu.ListGoKerPackages(t, "../..") {
		test := test
		_ = tests
		/*
			included := false
			for _, curated := range tests {
				if strings.HasPrefix(test, curated) && test == curated {
					included = true
					break
				}
			}

			if !included {
				continue
			}
		*/

		if strings.HasSuffix(test, "_fixed") {
			continue
		}

		t.Run(tu.GoKerTestName(test), func(t *testing.T) {
			t.Parallel()
			tu.ParallelHelper(t,
				tu.LoadExampleAsPackages(t, "../..", test, true),
				func(loadRes tu.LoadResult) {
					entry := loadRes.Mains[0].Func("main")
					if entry == nil {
						t.Fatal("Could not find main function on", loadRes.Mains[0])
					}

					callDAG := loadRes.PrunedCallDAG
					G := callDAG.Original

					computeDominator := G.DominatorTree(entry)

					_, primsToUses := gotopo.GetPrimitives(entry, loadRes.Pointer, G, false)

					// GCatch PSets
					psets := gotopo.GetGCatchPSets(
						loadRes.Cfg, entry, loadRes.Pointer, G,
						computeDominator, callDAG, primsToUses,
					)
					//psets := gotopo.GetSCCPSets(callDAG, primsToUses, loadRes.Pointer)
					/*
						pset := utils.MakeSSASet()
						for prim := range primsToUses {
							pset = pset.Add(prim)
						}
						psets = append(psets, pset)
					*/

					// Ensure consistent ordering
					sort.Slice(psets, func(i, j int) bool {
						return psets[i].String() < psets[j].String()
					})

					t.Log(len(psets), "PSets to process")

					blocks := make(Blocks)

					for i, pset := range psets {
						t.Log(color.CyanString("PSet"), i+1, color.CyanString("of"), len(psets), color.CyanString(":"))
						t.Log(pset)

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

						C := AIConfig{Metrics: true}.Function(loweredEntry)(loadRes)
						C.FragmentPredicateFromPrimitives(pset.Entries(), primsToUses)

						C.TimerStart()
						ts, analysis := StaticAnalysis(C)

						//log.Println("Superlocation graph size:", ts.Size())

						switch C.Outcome {
						case OUTCOME_PANIC:
							t.Log(color.RedString("Aborted!"))
							t.Log(C.Error())
						default:
							C.Done()
							t.Log(color.GreenString("SA completed in %s", C.Performance()))

							blocks.UpdateWith(BlockAnalysis(C, ts, analysis))
						}
					}

					findClInSl := func(ann tu.AnnProgress) func(defs.Goro, defs.CtrLoc) bool {
						return func(g defs.Goro, cl defs.CtrLoc) bool {
							if /* !ann.HasFocus() || ann.Focused().Matches(g) */ true {
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

					fnBefore, tpBefore := metrics.falseNegatives, metrics.truePositives
					fns := []string{}
					tu.MakeNotesManager(t, loadRes).ForEachAnnotation(func(a tu.Annotation) {
						if ann, ok := a.(tu.AnnBlocks); ok {
							/*
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
							*/

							if !blocks.Exists(findCl(ann)) {
								t.Log("False negative:", ann)
								fns = append(fns, ann.String())
								metrics.falseNegatives++
							} else {
								metrics.truePositives++
							}
						}
					})

					// Construct golden test output.
					// This helps us detect advances and regressions in detected bugs.
					fnCnt := metrics.falseNegatives - fnBefore
					tpCnt := metrics.truePositives - tpBefore

					var out bytes.Buffer
					sort.Strings(fns)
					fmt.Fprintf(&out, "TP: %d FN: %d\n%s\n", tpCnt, fnCnt, strings.Join(fns, "\n"))

					goldie.New(t).Assert(t, t.Name(), out.Bytes())

					if t.Failed() {
						t.Log("Detected blocks:\n", blocks)
					}
				},
			)
		})
	}

	// t.Logf("%+v", metrics)
}
