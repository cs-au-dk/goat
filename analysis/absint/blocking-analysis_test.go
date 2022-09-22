package absint

import (
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	loc "github.com/cs-au-dk/goat/analysis/location"
	"github.com/cs-au-dk/goat/pkgutil"
	tu "github.com/cs-au-dk/goat/testutil"

	"golang.org/x/tools/go/ssa/ssautil"
)

type blockAnalysisMetrics struct {
	truePositives, falsePositives, falseNegatives int
}

var metrics blockAnalysisMetrics

func BlockAnalysisTest(
	t *testing.T,
	C AnalysisCtxt,
	result L.Analysis,
	S SuperlocGraph,
	nmgr tu.NotesManager) {

	bs := BlockAnalysis(C, S, result)

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

	nmgr.ForEachAnnotation(func(a tu.Annotation) {
		switch ann := a.(type) {
		case tu.AnnBlocks:
			if !bs.Exists(findCl(ann)) {
				for sl, anns := range nmgr.OrphansToAnnotations(bs) {
					t.Logf("Blocked goroutines for %s", sl)
					for _, a := range anns {
						t.Log(a)
					}
					println()
				}

				if ann.FalseNegative() {
					t.Log("False negative:", ann)
					metrics.falseNegatives++
				} else if _, _, found := result.Find(func(sl defs.Superloc, _ L.AnalysisState) bool {
					_, _, innerFound := sl.Find(findClInSl(ann))
					return innerFound
				}); !found {
					t.Error("The analysis result does not contain data-flow for a superlocation that matches", ann)
				} else {
					t.Errorf("Expected blocked goroutine with annotation %s", ann)
				}
			} else {
				metrics.truePositives++
			}
		case tu.AnnReleases:
			if sl, _, found := bs.Find(findCl(ann)); found {
				g, _, found := sl.Find(findClInSl(ann))
				if !found {
					t.Fatalf("Could not identify matching goroutine for %s?", ann)
				}

				mem := result.GetUnsafe(sl).Memory()
				cl := sl.GetUnsafe(g)

				// Print some information about the operands of the instruction the goroutine is blocked at
				explanation := ""
				if ssaNode, ok := cl.Node().(*cfg.SSANode); ok {
					for _, oper := range ssaNode.Instruction().Operands(nil) {
						val := evaluateSSA(g, mem, *oper)
						explanation = fmt.Sprintf("%s\n%s = %v\n", explanation, (*oper).Name(), val)

						if val.IsPointer() {
							mops := L.MemOps(mem)
							for _, ptr := range val.PointerValue().FilterNil().Entries() {
								if _, isFun := ptr.(loc.FunctionPointer); !isFun {
									explanation = fmt.Sprintf("%s%v ↦ %v\n", explanation, ptr, mops.GetUnsafe(ptr))
								}
							}
						}
					}
				}

				t.Log(g, "blocked in", sl)
				if ann.FalsePositive() {
					t.Logf("False positive: %s\n%s", ann, explanation)
				} else {
					t.Errorf("Expected no blocked goroutine with annotation %s\n%s", ann, explanation)
				}
			} else if _, _, found := result.Find(func(sl defs.Superloc, _ L.AnalysisState) bool {
				_, _, innerFound := sl.Find(findClInSl(ann))
				return innerFound
			}); !found {
				t.Error("The analysis result does not contain data-flow for a superlocation that matches", ann)
			}
		}
	})

	// Identify false positives assuming every true positive is annotated with //@ blocks
	blocksNotes := nmgr.FindAllAnnotations(func(ann tu.Annotation) bool {
		_, isBlocks := ann.(tu.AnnBlocks)
		return isBlocks
	})

	bs.ForEach(func(sl defs.Superloc, gs map[defs.Goro]struct{}) {
		for g := range gs {
			if !blocksNotes.Exists(func(ann tu.Annotation) bool {
				return findClInSl(ann.(tu.AnnBlocks))(g, sl.GetUnsafe(g))
			}) {
				t.Logf("False positive:\n%s\n%s %s", sl, g, sl.GetUnsafe(g).Node().Function())
				metrics.falsePositives++
			}
		}
	})
}

// Create spawning point names
func g(i int) string {
	return "go" + strconv.Itoa(i)
}

// Create channel names
func ch(i int) string {
	return "ch" + strconv.Itoa(i)
}

// Create goroutine name
func goro(i int) string {
	return "goro" + strconv.Itoa(i)
}

func TestBlockingAnalysis(t *testing.T) {
	const (
		main = "main"
		root = "_root"
	)
	releases := ann.MayRelease("")
	blocks := ann.Blocks("")

	tests := []absIntCommTest{
		{
			"sync-parent-child",
			`func main() {
				ch := make(chan int)
				go func() {
					<-ch` + at(releases) + `
				}()
				ch <- 10` + at(releases) + `
			}`,
			BlockAnalysisTest,
		}, {
			"sync-siblings",
			`func main() {
				ch := make(chan int)
				go func() {
					<-ch` + at(releases) + `
				}()
				go func() {
					ch <- 10` + at(releases) + `
				}()
			}`,
			BlockAnalysisTest,
		}, {
			"basic-orphan-send",
			`func main() {
					ch := make(chan string)
					go func() {
						ch <- "basic-orphan"` + at(blocks) + `
					}()
				}`,
			BlockAnalysisTest,
		}, {
			"basic-orphan-send-buffer-fix",
			`func main() {
					ch := make(chan string, 1)
					go func() {
						ch <- "basic-orphan-fix"` + at(releases) + `
					}()
				}`,
			BlockAnalysisTest,
		}, {
			"basic-orphan-receive",
			`func main() {
				ch := make(chan string)
				go func() {
					<-ch` + at(blocks) + `
				}()
			}`,
			BlockAnalysisTest,
		}, {
			"basic-orphan-receive-close-fix",
			`func main() {
				ch := make(chan string)
				go func() {
					<-ch` + at(releases) + `
				}()
				close(ch)
			}`,
			BlockAnalysisTest,
		}, {
			"basic-vilomah-send",
			`func main() {
				ch := make(chan string)
				go func() {
				}()
				ch <- "basic-orphan"` + at(blocks) + `
			}`,
			BlockAnalysisTest,
		}, {
			"basic-vilomah-receive",
			`func main() {
				ch := make(chan string)
				go func() {
				}()
				<-ch` + at(blocks) + `
			}`,
			BlockAnalysisTest,
		}, {
			"basic-vilomah-receive-close-fix",
			`func main() {
				ch := make(chan string)
				go func() {
					close(ch)
				}()
				<-ch` + at(releases) + `
			}`,
			BlockAnalysisTest,
		}, {
			"dub-orphans",
			`func main() {
				ch := make(chan string)
				go func() {
					ch <- "dub-orphans"` + at(blocks) + `
				}()
				go func() {
					ch <- "dub-orphans"` + at(blocks) + `
				}()
				go func() {
					ch <- "dub-orphans"` + at(blocks) + `
				}()
			}`,
			BlockAnalysisTest,
		}, {
			"dub-orphans-partial-fix",
			`func main() {
				ch := make(chan string, 2)
				go func() {
					ch <- "dub-orphans-partial-fix"` + at(blocks) + `
				}()
				go func() {
					ch <- "dub-orphans-partial-fix"` + at(blocks) + `
				}()
				go func() {
					ch <- "dub-orphans-partial-fix"` + at(blocks) + `
				}()
			}`,
			BlockAnalysisTest,
		}, {
			"dub-orphans-fix",
			`func main() {
					ch := make(chan string, 3)
					go func() {
						ch <- "dub-orphans-fix"` + at(releases) + `
					}()
					go func() {
						ch <- "dub-orphans-fix"` + at(releases) + `
					}()
					go func() {
						ch <- "dub-orphans=fix"` + at(releases) + `
					}()
				}`,
			BlockAnalysisTest,
		}, {
			"select-orphan",
			`func main() {
					ch := make(chan string)
					go func() {
						ch <- "select-orphan"` + at(blocks) + `
					}()
					select {
					case <-ch:
					default:
					}
				}`,
			BlockAnalysisTest,
		}, {
			"same-goro-loc",
			`func e() {
					go func() {
						make(chan string) <- "same-goro-loc"` + at(blocks) + `
					}()
				}

				func main() {
					e()
					e()
				}`,
			BlockAnalysisTest,
		}, {
			"same-goro-loc-param",
			`func e(ch chan string) {
					go func() {
						ch <- "same-goro-loc"` + at(blocks) + `
					}()
				}

				func main() {
					ch := make(chan string)
					e(ch)
					e(ch)
				}`,
			BlockAnalysisTest,
		}, {
			"same-goro-loc-param-buf",
			// The analysis will fail to capture a bug if GORO_BOUND is set to 1.
			// It will capture it if the bound is raised to 2.
			`func e(ch chan string) {
				go func() {` + at(
				ann.Go(g(1)),
				ann.FocusedChanQuery(ch(0), tu.QRY_CAP, 1, main, goro(1)),
			) + `
					ch <- "same-goro-loc-param-buf"` + at(func() string {
				if opts.ExceedsGoroBound(2) {
					return releases
				}
				return blocks
			}()) + `
				}()
			}

			` + at(
				ann.Goro(goro(1), false, g(1)),
				ann.Goro(main, true, root),
			) + `
			func main() {
				ch := make(chan string, 1)` + at(ann.Chan(ch(0))) + `
				e(ch)
				e(ch)
			}`,
			StaticAnalysisAndBlockingTests,
		}, {
			"same-goro-loc-forked",
			`func e(x string) {
					go func() {
						make(chan string) <- x` + at(blocks) + `
					}()
				}

				func main() {
					if func() bool {
						ch := make(chan int, 1)
						select {` + at(releases) + `
						case ch <- 0:
							return true
						default:
							return false
						}
					}() {
						e("same-goro-loc-forked-1")
					} else {
						e("same-goro-loc-forked-2")
					}
				}`,
			BlockAnalysisTest,
		}, {
			"select-orphan-fix",
			`func main() {
						ch := make(chan string, 1)
						go func() {
							ch <- "select-orphan-fix"` + at(releases) + `
						}()
						select {
						case <-ch:
						default:
						}
					}`,
			BlockAnalysisTest,
		}, {
			"one-blocks",
			at(
				ann.Goro(goro(0), true, root, g(0)),
				ann.Goro(goro(1), true, root, g(1)),
			) + `
			func a(ch chan int) {
				ch <- 0` + at(
				ann.Blocks(goro(0)),
				ann.MayRelease(goro(1)),
			) + `
			}

			func main() {
				go a(make(chan int))` + at(ann.Go(g(0))) + `
				go a(make(chan int, 1))` + at(ann.Go(g(1))) + `
			}`,
			BlockAnalysisTest,
		}, {
			// Need proper cycle detection for the main thread
			"[disabled] starvation-issue",
			at(ann.Goro(goro(0), true, root, g(0))) + `
			func main() {
				for {
					go func() { ` + at(ann.Go(g(0))) + `
						<-make(chan int) ` + at(ann.Blocks(goro(0))) + `
					}()
				}
			}`,
			BlockAnalysisTest,
		}, {
			"livelocked-orphans",
			`func main() {
				ch := make(chan string)
				go func() {
					for {
						ch <- "cyclic-orphans"
					}
				}()
				go func() {
					for {
						<-ch
					}
				}()
			}`,
			BlockAnalysisTest,
		}, {
			"livelocked-non-orphan",
			`func main() {
				ch := make(chan string)
				go func() {
					for {
						ch <- "cyclic-orphans-fix"
					}
				}()
				for {
					<-ch
				}
				<-make(chan string)
			}`,
			BlockAnalysisTest,
		}, {
			"infinite-loop-soundness-issue",
			at(ann.Goro(goro(0), true, root, g(0))) + `
			func main() {
				ch := make(chan int)
				go func() { ` + at(ann.Go(g(0))) + `
					ch <- 10 ` + at(ann.Blocks(goro(0)), ann.FalseNegative()) + `
				}()

				// Get a top value
				sl := []bool{true, false}
				top := sl[0]

				if top {
					<-ch
				}

				for { }
			}`,
			BlockAnalysisTest,
		}, {
			"dingo-hunter-3",
			`func Send(ch chan<- int) { ch <- 42 }

			func Recv(ch <-chan int, done chan<- int) {
				val := <-ch` + at(ann.Blocks(g(1)), ann.Blocks(g(2))) + `
				done <- val
			}

			func Work() {
				for {
					_ = func () bool { return true }()
			 	}
			}
			
			` + at(
				ann.Goro(main, false, main),
				ann.Goro(g(0), false, g(0)),
				ann.Goro(g(1), false, g(1)),
				ann.Goro(g(2), false, g(2))) + `

			func main() {` + at(ann.Go(main)) + `
			 	ch, done := make(chan int), make(chan int)
			
			 	go Send(ch)` + at(ann.Go(g(0))) + `
			 	go Recv(ch, done)` + at(ann.Go(g(1))) + `
			 	go Recv(ch, done)` + at(ann.Go(g(2))) + `
			 	go Work()
			
			 	<-done
			 	<-done` + at(blocks) + `
			}`,
			BlockAnalysisTest,
		}, {
			"debug-grpc-862-fixed",
			`var closedchan chan int = func() chan int {
				c := make(chan int)
				close(c)
				return c
			}()

			` + at(ann.Goro(main, true, root)) + `
			func main() {
				var c chan int
				switch []int{0, 1}[0] {
				case 0:
					c = closedchan
				case 1:
					for i := 0; i < 2; i++ {
						maybeClosed := make(chan int)
						close(maybeClosed)
						c = maybeClosed
					}
				}

				<-c ` + at(ann.MayRelease(main)) + `
			}`,
			BlockAnalysisTest,
		}, {
			"mutex-test",
			`import "sync"

			` + at(ann.Goro(main, true, root)) + `
			func main() {
				var mu sync.Mutex
				mu.Lock() ` + at(ann.MayRelease(main)) + `
			}`,
			BlockAnalysisTest,
		},
		{
			"matching-for-loops",
			at(ann.Goro(main, true, root),
				ann.Goro(g(0), true, root, g(0))) + `
			func main() {
				arr := make([]int, func() int { return 10 }())
				ch := make(chan int, len(arr))
				for _, el := range arr {
					go func(x int) { ` + at(ann.Go(g(0))) + `
						ch <- x ` + at(ann.MayRelease(g(0))) + `
					}(el)
				}

				for _, el := range arr {
					println(<-ch == el) ` + at(ann.MayRelease(main)) + `
				}
			}`,
			BlockAnalysisTest,
		},
		{
			"write-then-close-with-wg",
			`import "sync"
			` + at(ann.Goro(main, true, root),
				ann.Goro(g(0), true, root, g(0))) + `
			func main() {
				errch := make(chan error, 1)
				var wg sync.WaitGroup
				wg.Add(1)
				go func() { ` + at(ann.Go(g(0))) + `
					defer wg.Done()
					errch <- nil ` + at(ann.MayRelease(g(0))) + `
				}()

				wg.Wait()
				close(errch)

				for err := range errch { ` + at(ann.MayRelease(main)) + `
					println(err)
				}
			}`,
			BlockAnalysisTest,
		},
		{
			"runtime-gooexit",
			`import "runtime"

			func callExit() {
				runtime.Goexit()
			}

			` + at(ann.Goro(main, true, root)) + `
			func main() {
				ch := make(chan int)
				// Test that Goexit unwinds the defer stack, unblocking main
				go func() {
					defer close(ch)
					callExit()
				}()
				<-ch ` + at(ann.MayRelease(main)) + `

				// Test that Goexit will stop regular execution, leading to a block
				ch = make(chan int)
				go func() {
					callExit()
					ch <- 10
				}()
				<-ch ` + at(ann.Blocks(main)) + `
			}`,
			BlockAnalysisTest,
		},
		{
			// See TODO in absint of FunctionExit
			"[disabled] comm-separated-calls",
			at(ann.Goro(main, true, root), ann.Goro(g(0), true, root, g(0))) + `
			func helper() {}
			func main() {
				ch := make(chan int)
				go func() { ` + at(ann.Go(g(0))) + `
					ch <- 10 ` + at(ann.MayRelease(g(0))) + `
				}()

				helper()
				<-ch ` + at(ann.MayRelease(main)) + `
				helper()
			}`,
			BlockAnalysisTest,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if strings.HasPrefix(test.name, "[disabled]") {
				t.SkipNow()
			}

			runEmbeddedTest(t, test)
		})
	}

	t.Run("abort-on-t.Fatal", func(t *testing.T) {
		pkgs, err := pkgutil.LoadPackages(pkgutil.LoadConfig{
			GoPath:       "../../examples",
			ModulePath:   "../../examples/src/pkg-with-test",
			IncludeTests: true,
		}, "pkg-with-test")
		if err != nil {
			t.Fatal("Failed to load packages:", err)
		}

		if len(pkgs) != 3 {
			t.Fatal("Expected 3 packages, got:", len(pkgs), pkgs)
		}

		pkgs = pkgs[1:]

		prog, ssaPkgs := ssautil.AllPackages(pkgs, 0)
		prog.Build()

		testFun := ssaPkgs[0].Func("TestFatal")
		if testFun == nil {
			t.Fatal("Unable to find 'TestFatal' in", ssaPkgs[0])
		}

		loadRes := tu.LoadResultFromPackages(t, pkgs)
		manager := tu.MakeNotesManager(t, loadRes)

		notes := manager.Notes()
		if len(notes) != 1 {
			t.Fatal("Expected exactly 1 note, got:", notes)
		}

		defer func() {
			if err := recover(); err != nil {
				t.Errorf("Panic while analyzing...\n%v\n%s\n", err, debug.Stack())
			}
		}()

		C := PrepareAI().FunctionByName("TestFatal", true)(loadRes)
		C.setFragmentPredicate(false, true)
		S, result := StaticAnalysis(C)

		BlockAnalysisTest(t, C, result, S, manager)
	})
}
