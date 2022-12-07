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
	tu "github.com/cs-au-dk/goat/testutil"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/graph"

	"github.com/benbjohnson/immutable"
)

var at = tu.At
var ann = tu.Ann

type absIntCommTestFunc func(
	*testing.T,
	AnalysisCtxt,
	L.Analysis,
	SuperlocGraph,
	tu.NotesManager)
type absIntCommTest = struct {
	name, content string
	fun           absIntCommTestFunc
}

// Caller should ensure that runWholeProgTest is called in a subtest (inside t.Run(...))
func runWholeProgTest(
	t *testing.T,
	loadRes tu.LoadResult,
	fun absIntCommTestFunc) {
	runTest(t, loadRes, fun, func(loadRes tu.LoadResult) AnalysisCtxt {
		ctxt := PrepareAI().WholeProgram(loadRes)
		// The static analysis tests are small tests where we want to explore all calls
		// TODO: Also the current heuristic for spoofing unrelated calls causes
		// us to miss dataflow to blocks/releases annotations
		//ctxt.AnalyzeCallsWithoutConcurrencyPrimitives = true
		ctxt.setFragmentPredicate(false, true)
		return ctxt
	})
}

func runTest(
	t *testing.T,
	loadRes tu.LoadResult,
	fun absIntCommTestFunc,
	prep func(tu.LoadResult) AnalysisCtxt) {
	nmgr := func() tu.NotesManager {
		defer func() {
			if err := recover(); err != nil {
				t.Errorf("Panic while building notes...\n%v\n%s\n", err, debug.Stack())
			}
		}()

		return tu.MakeNotesManager(t, loadRes)
	}()

	defer func() {
		if err := recover(); err != nil {
			t.Errorf("Panic while analyzing...\n%v\n%s\n", err, debug.Stack())
		}
	}()

	C := prep(loadRes)
	S, result := StaticAnalysis(C)

	t.Logf("Abstract configuration graph contains %d superlocations.", result.Size())

	// Compute some stats on how many superlocations we visit out of the total possible
	goroToClCount := utils.NewImmMap[defs.Goro, *immutable.Map[defs.CtrLoc, int]]()
	visited := 0
	S.ForEach(func(conf *AbsConfiguration) {
		if conf.IsPanicked() || !conf.IsSynchronizing(C, result.GetUnsafe(conf.Superlocation())) {
			return
		}

		visited++

		conf.ForEach(func(g defs.Goro, cl defs.CtrLoc) {
			prevCnt, ok := goroToClCount.Get(g)
			if !ok {
				prevCnt = utils.NewImmMap[defs.CtrLoc, int]()
			}

			pCnt, _ := prevCnt.Get(cl)
			goroToClCount = goroToClCount.Set(g, prevCnt.Set(cl, pCnt+1))
		})
	})

	if visited > 0 {
		totalPossible := 1
		for iter := goroToClCount.Iterator(); !iter.Done(); {
			_, clCnt, _ := iter.Next()
			presentCnt := 0
			for iter := clCnt.Iterator(); !iter.Done(); {
				_, v, _ := iter.Next()
				presentCnt += v
			}

			myPossible := clCnt.Len()
			if presentCnt < visited {
				// If the goroutine is not present in all synchronizing superlocations we
				// add 1 to the number of possibilities for this goroutine
				myPossible++
			}

			totalPossible *= myPossible
		}

		t.Logf("Visited %d/%d (%.2f%%) of all possible synchronizing superlocations",
			visited, totalPossible, (float64(visited)*100)/float64(totalPossible))
	} else {
		t.Log("Visited 0 synchronizing superlocations")
	}

	if fun != nil {
		fun(t, C, result, S, nmgr)
	}
}

func runEmbeddedTest(t *testing.T, test absIntCommTest) {
	runWholeProgTest(t,
		tu.LoadPackageFromSource(t, "testpackage", "package main\n\n"+test.content),
		test.fun,
	)
}

func ChannelValueQueryTests(
	t *testing.T,
	_ AnalysisCtxt,
	res L.Analysis,
	S SuperlocGraph,
	mgr tu.NotesManager) {

	mgr.ForEachAnnotation(func(a tu.Annotation) {
		switch a := a.(type) {
		case tu.AnnChanQuery:
			mkchan := a.Chan()

			checkedOne := false

			for node := range a.Nodes() {
				res.ForEach(func(s defs.Superloc, as L.AnalysisState) {
					// Don't check panicked superlocations
					if _, _, isPanicked := s.Find(func(_ defs.Goro, cl defs.CtrLoc) bool {
						return cl.Panicked()
					}); isPanicked {
						return
					}

					g, cl, found := s.Find(func(g defs.Goro, cl defs.CtrLoc) bool {
						// If the channel query focuses on a specific goroutine,
						// eliminate goroutines that do not match.
						if a.IsFocused() && !a.FocusedNote().Matches(g) {
							return false
						}

						// Found a control location that matches the node.
						return cl.Node() == node
					})

					if !found {
						return
					}

					mem := as.Memory()
					errStr := func(al loc.AllocationSiteLocation, val L.Element) string {
						return fmt.Sprintf("Expected `%s` field of channel %s to be: %s\n"+
							"Instead found: %s\n"+
							"Goroutine: %s\n"+
							"Control location: %s\n"+
							"Superlocation: %s\n"+
							"Whole memory: %s",
							a.Prop(), al, a.Value(),
							val, g.CtrLoc(), cl, s,
							mem,
						)
					}

					mem.ForEach(func(al loc.AddressableLocation, av L.AbstractValue) {
						asl, ok := al.(loc.AllocationSiteLocation)
						if !ok {
							return
						}

						// If the channel query specifies an owning goroutine,
						// eliminate searches on locations which are not owned by
						// the same goroutine in the result.
						if a.HasOwner() && !a.GownerNote().Matches(asl.Goro.(defs.Goro)) {
							return
						}

						if asl.Site == mkchan {
							ach := av.ChanValue()
							var val L.Element
							switch a.Prop() {
							case tu.QRY_MULTIALLOC:
								val = L.Create().Element().TwoElement(mem.IsMultialloc(asl))
							case tu.QRY_STATUS:
								val = ach.Status()
							case tu.QRY_CAP:
								val = ach.Capacity()
							case tu.QRY_BUFFER_F:
								val = ach.BufferFlat()
							case tu.QRY_BUFFER_I:
								val = ach.BufferInterval()
							}
							if !a.Value().Eq(val) {
								t.Errorf(errStr(asl, val))
							}
							checkedOne = true
						}
					})
				})
			}

			if !checkedOne {
				t.Errorf("No abstract channel found to match the given query: %s", a)
			}
		}
	})
}

func TestStaticAnalysis(t *testing.T) {
	type i = struct {
		L int
		H int
	}

	// Create spawning point names
	g := func(i int) string {
		return "go" + strconv.Itoa(i)
	}
	// Crewate channel names
	ch := func(i int) string {
		return "ch" + strconv.Itoa(i)
	}
	// Create goroutine name
	goro := func(i int) string {
		return "goro" + strconv.Itoa(i)
	}

	// Some channel and goroutine name constants,
	// to alleviate query typos.
	const (
		main = "main"
		root = "_root"
	)
	var (
		go1 = g(1)
		go2 = g(2)

		ch1 = ch(1)
		// ch2 = ch(2)
	)

	tests := []absIntCommTest{
		{
			"simple-value",
			`func main() {
					x := 8
					_ = x
				}`,
			ChannelValueQueryTests,
		}, {
			"spawn-goroutine",
			`func main() {
						go func() {}()
					}`,
			nil,
		}, {
			"make-buff-chan-const",
			`func main() {
				<-make(chan int, 1)` + at(
				ann.Chan(ch1),
				ann.ChanQuery(ch1, tu.QRY_MULTIALLOC, false),
				ann.ChanQuery(ch1, tu.QRY_CAP, 1),
				ann.ChanQuery(ch1, tu.QRY_BUFFER_F, 0),
				ann.ChanQuery(ch1, tu.QRY_BUFFER_I, i{0, 0}),
			) + `
			}`,
			ChannelValueQueryTests,
		}, {
			"send-on-buff-chan",
			`func main() {
				ch := make(chan int, 1) ` + at(ann.Chan(ch1)) + `
				ch <- 10` + at(ann.ChanQuery(ch1, tu.QRY_CAP, 1)) + `
				make(chan string) <- "cheating"` + at(ann.ChanQuery(ch1, tu.QRY_BUFFER_I, i{1, 1})) + `
			}`,
			ChannelValueQueryTests,
		}, {
			"close-chan",
			`func main() {
				ch := make(chan int)` + at(ann.Chan(ch1)) + `
				close(ch)
				ch <- 10` + at(ann.ChanQuery(ch1, tu.QRY_STATUS, false)) + `
			}`,
			ChannelValueQueryTests,
		}, {
			"close-nil-chan",
			`func main() {
				var ch chan int
				close(ch)
			}`,
			nil,
		}, {
			"gowner-test",
			at(ann.Goro(goro(1), true, root, go1)) + `

			func main() {
				go func() {` + at(ann.Go(go1)) + `
					ch1 := make(chan string)` + at(ann.Chan(ch1)) + `
					go func () {` + at(ann.Go(go2)) + `
						ch1 <- "gowner-test"
					}()
					<-ch1
				}()
				<-make(chan string)` + at(
				// If the focused query names "go2" as the owner, the search should fail
				ann.FocusedChanQuery(ch1, tu.QRY_MULTIALLOC, false, goro(1), ""),
			) + `
			}`,
			ChannelValueQueryTests,
		}, {
			"focused-query-test",
			at(
				ann.Goro(main, true, root),
				ann.Goro(goro(1), false, go1),
				ann.Goro(goro(2), false, go2),
			) + `

			func exec(ch chan string, str string) {
				ch <- str ` + at(
				ann.FocusedChanQuery(ch1, tu.QRY_BUFFER_I, i{0, 0}, main, goro(1)),
				ann.FocusedChanQuery(ch1, tu.QRY_BUFFER_I, i{1, 1}, main, goro(2)),
			) + `
			}

			func main() {
				ch := make(chan string, 2)` + at(ann.Chan(ch1)) + `
				go func() {` + at(ann.Go(go1)) + `
					exec(ch, "first")
					go func() {` + at(ann.Go(go2)) + `
						exec(ch, "second")
					}()
				}()
			}`,
			ChannelValueQueryTests,
		}, {
			"invalid-cycle-gc-issue",
			`func helper() {}
			func f(i int) {
				x := i * i
				helper()
				println(x)
			}

			func main() {
				ch := make(chan int)
				go func() {
					ch <- 10
				}()

				f(42)
				<-ch
				helper()
			}`,
			nil,
		},
		{
			// Reproducer for the charge removal issue previously described in
			// absint.go for FunctionExit.
			"no-charged-return-issue",
			`func h() {}
			func g() { h() }
			func f() { h(); g() }

			func main() {
				f()
				// The call site in f calling h is still charged due to
				// being propagated on return from h to g.
				make(chan int, 1) <- 0  // Reset information at function entries
				// The return from h returns into f (and main), but f cannot return to main
				// as the call site in main to f is not charged.
				h()
			}`,
			nil,
		},
		{
			// Reproducer showing why need to remove all charged edges leaving
			// the exiting component, not just the ones from the exiting function.
			"no-charged-return-issue-cycle",
			`func isEven(x int) bool {
				if x == 0 {
					return true
				}
				return isOdd(x-1)
			}
			func isOdd(x int) bool {
				if x == 0 {
					return false
				}
				return isEven(x-1)
			}
			func g() { isOdd(10) }
			func f() { isEven(10); g() }

			func main() {
				f()
				// The call site in f calling isEven shouldn't be charged here
				make(chan int, 1) <- 0  // Reset information
				isEven(12)
			}`,
			nil,
		},
		{
			// We should be careful to only remove edges when returning out of a component...
			"no-charged-return-issue-cycle-2",
			`func isEven(x int) bool {
				if x == 0 {
					return true
				}
				return isOdd(x-1)
			}
			func isOdd(x int) bool {
				if x == 0 {
					return false
				}
				ans := isEven(x-1)
				// Since isEven returns into the component {isEven, isOdd}, we
				// shouldn't remove the charged return edge isEven -> main.
				make(chan int, 1) <- 0 // Reset information
				return ans
			}
			func main() { isEven(10) }`,
			nil,
		},
		{
			"no-charged-return-issue-lca",
			`func a() { b(); c() }
			func b() { d() }
			func c() { d() }
			func d() { }
			func main() {
				a()
				make(chan int, 1) <- 0
				b()
			}`,
			nil,
		},
		{
			"remove-charged-return-increases-precision",
			`var x int
			func f() {}
			func main() {
				f()
				x = 10
				make(chan int, 1) <- 0  // reset stored information at f-entry
				f()
			}`,
			func(
				t *testing.T,
				C AnalysisCtxt,
				A L.Analysis,
				S SuperlocGraph,
				_ tu.NotesManager,
			) {
				anyFound := false
				global := C.Function.Pkg.Var("x")
				if global == nil {
					t.Fatal("No global named 'x'")
				}

				A.ForEach(func(sl defs.Superloc, a L.AnalysisState) {
					sl.ForEach(func(_ defs.Goro, cl defs.CtrLoc) {
						if _, ok := cl.Node().(*cfg.TerminateGoro); ok {
							anyFound = true
							val := a.Memory().GetUnsafe(loc.GlobalLocation{Site: global})
							if !val.Eq(L.Elements().AbstractBasic(int64(10))) {
								t.Errorf("Expected 'x' %v = 10", val)
							}
						}
					})
				})

				if !anyFound {
					t.Error("Didn't find a superlocation where the goroutine terminated.")
				}
			},
		},
		{
			// This example shows why it is necessary to store return edges (with from and to)
			// instead of only return "points" (to only)
			// (assuming that all charged returns from a function are removed when it returns).
			"subtle-charged-return-example",
			`func ubool() bool
			func h() {}
			func g() {
				h()
				make(chan int, 1) <- 0
			}
			func main() {
				f := h
				if ubool() { f = g }
				f()
			}`,
			nil,
		},
		{
			"charged-return-sound-callgraph",
			chargedReturnSoundCallgraphProgram,
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runEmbeddedTest(t, test)
		})
	}
}

var chargedReturnSoundCallgraphProgram = func() string {
	aFuns := []string{}
	aNames := []string{}
	aCalls := []string{}
	for i := 0; i < graph.CGPruneLimit+1; i++ {
		name := fmt.Sprintf("a_%d", i)
		aFuns = append(aFuns, fmt.Sprintf(`
func %s(f func(chan int)) {
	f(nil)
}`, name))
		aCalls = append(aCalls, name+"(f)")
		aNames = append(aNames, name)
	}

	return fmt.Sprintf(`
%s

var as = []func(func(chan int)){ %s }

func f(ch chan int) {
	if !guard {
		guard = true
		as[0](f)
	}
	ch <- 10
}

var guard = false
func main() {
	if guard {
		%s
	}

	ch := make(chan int, 10)
	f(ch)
}
`, strings.Join(aFuns, "\n"), strings.Join(aNames, ", "), strings.Join(aCalls, "; "))
}()

func TestPOR(t *testing.T) {
	var PORTest absIntCommTestFunc = func(t *testing.T, C AnalysisCtxt,
		result L.Analysis, sg SuperlocGraph, notes tu.NotesManager) {

		t.Logf("Visited %d states.", result.Size())

		if len(notes.Notes()) != 0 {
			BlockAnalysisTest(t, C, result, sg, notes)
		}
	}

	tests := []absIntCommTest{
		{
			"must-consider-both-interleavings",
			`func prog() {
				ch1, ch2 := make(chan int), make(chan int)
				chbuf := make(chan int, 1)
				chbuf <- 0

				go func() {
					ch1 <- 10
					<-chbuf //@ blocks
				}()

				go func() {
					ch2 <- 10
					<-chbuf //@ blocks
				}()

				select {
				case <-ch1:
				case <-ch2:
				}
				select {
				case <-ch1:
				case <-ch2:
				}
			}

			func main() {
				prog()
			}`,
			PORTest,
		},
		{
			"not-independent",
			`func main() {
				ch := make(chan int)
				chbuf := make(chan int, 1)
				chbuf <- 0

				go func() {
					ch <- 42
				}()

				go func() {
					<-ch
					<-chbuf //@ blocks
				}()

				go func() {
					<-chbuf //@ blocks
				}()
			}`,
			PORTest,
		},
	}

	for i := 1; i <= 10; i++ {
		s := `func prog() {
			ch := make(chan int)

			go func() {
				ch <- 10
			}()

			<-ch
		}

		func main() {
		` + strings.Repeat("\tgo prog()\n", i) +
			`}`

		tests = append(tests, absIntCommTest{fmt.Sprintf("independent-parent-child-1-comm-%d", i), s, PORTest})
	}

	for i := 1; i <= 7; i++ {
		s := `func prog(done chan int) {
			ch := make(chan int)

			go func() {
				ch <- 10
			}()

			<-ch
			done <- 42
		}
		func main() {
			done := make(chan int)
		` + strings.Repeat("\tgo prog(done)\n", i) +
			strings.Repeat("\t<-done\n", i) +
			`}`

		tests = append(tests, absIntCommTest{fmt.Sprintf("maybe-independent-1-comm-%d", i), s, PORTest})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runEmbeddedTest(t, test)
		})
	}
}
