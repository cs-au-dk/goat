package analysis

import (
	"strings"
	"testing"

	"runtime/debug"

	ai "github.com/cs-au-dk/goat/analysis/absint"
	tu "github.com/cs-au-dk/goat/testutil"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/expect"
)

func analyzePackage(t *testing.T, pkg string) {
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("Panic while analyzing...\n%v\n%s\n", err, debug.Stack())
		}
	}()

	loadRes := tu.LoadExamplePackage(t, "..", pkg)
	// fset := loadRes.Prog.Fset

	nmgr := tu.MakeNotesManager(t, loadRes)

	// Map from note group ID to note index.
	// Used to group notes together where only one of them are guaranteed to be hit.
	noteGroups := make(map[int64]int)
	notes := []*expect.Note{}
	for note := range nmgr.FindAllNotes(func(n *expect.Note) bool {
		return n.Name == "analysis"
	}) {
		notes = append(notes, note)
	}

	foundMatch := make([]bool, len(notes))
	for i, note := range notes {
		if len(note.Args) >= 2 {
			groupID := note.Args[1].(int64)

			// Check if this note is the first in the group
			if _, found := noteGroups[groupID]; !found {
				noteGroups[groupID] = i
			} else {
				// Do not require this note to be found
				foundMatch[i] = true
			}
		}
	}

	// Progress the program concretely
	//analysis.ResetThreadCounter()
	//c0 := analysis.SingleThreadMakeProgressConcrete("0", prog_cfg.GetEntries()[0], []*ssa.Function{})
	//c0 := analysis.DebuggerFrontend(*prog_cfg)
	//s0 := c0.Abstract(analysis.ABS_COARSE).(*analysis.AbsConfiguration)
	//initMem := c0.Memory()

	// Try to avoid the debugger frontend

	// Try to progress the state a bit further to catch more notes.
	// s0, initState = analysis.CoarseProgress(s0, initState, analysis.Etc{})

	analysisContext := ai.PrepareAI().WholeProgram(loadRes)
	slocG, result := ai.StaticAnalysis(analysisContext)
	if utils.Opts().Visualize() {
		slocG.Visualize(nil)
	}

	t.Logf("Abstract configuration graph contains %d superlocations.", result.Size())

	/*
		var liveVars lattice.Element = nil
		// For each thread, check if it is stopped at a cfg node with a note.
		// Start analysis if that is the case.
		s0.ForEach(func(g defs.Goro, loc defs.CtrLoc) {
			if !loc.Node().IsCommunicationNode() {
				return
			}
			var ppos token.Pos
			switch cfgNode := loc.Node().(type) {
			case *cfg.SSANode:
				ppos = cfgNode.Instruction().Pos()
			case *cfg.Select:
				ppos = cfgNode.Insn.Pos()
			default:
				return
			}

			pos := fset.Position(ppos)

			nmgr.ForEach(func(i int, note *expect.Note) {
				expectLive := note.Args[0].(bool)
				// Get the right index to mark in case the note is part of a group
				if len(note.Args) >= 2 {
					i = noteGroups[note.Args[1].(int64)]
				}

				npos := fset.Position(note.Pos)
				// Heuristic to match comment position with ssa instruction
				if pos.Line == npos.Line && pos.Column <= npos.Column {

					foundMatch[i] = true
					t.Logf("Matched %v with %v\n", note.Pos, loc.Node())

					C := analysis.ComputeInitialRelevantChannels(s0, g, initState.Memory())

					// Delayed initialization of liveVars in case we never start the analysis
					if liveVars == nil && false {
						liveVars = livevars.LiveVars(*loadRes.Cfg, loadRes.Pointer)
					}

					for {
						slocG, _ := analysis.StaticAnalysis(s0, initState)
						if utils.Opts.Visualize {
							slocG.Entry().Visualize()
						}

						break

						/*
							s0, analysisResult, confToMem := analysis.AnalysisBackend(s0, initState, analysis.Etc{
								Chans: C, PointsTo: loadRes.Pointer, Target: g, LiveVars: liveVars,
							})


							livenessGraph := analysis.CreateLivenessGraph(s0, C, confToMem, analysisResult)

							counterexamples := analysis.FindLivenessCounterexamples(livenessGraph)

							if utils.Opts.Visualize {
								livenessGraph.Visualize(counterexamples)
							}

							isLive := len(counterexamples) == 0

							// If isLive == expectLive == false, we should still attempt to do some refinements.
							//  This could catch soundness bugs that only show up after refinements.
							// TODO: Set a bound on the number of refinements.
							if isLive {
								if !expectLive {
									t.Errorf("Soundness error. Operation was unexpectedly live.")
								}

								break
							}

							var changed bool
							C, changed = analysis.FindRefinement(livenessGraph, counterexamples, C, confToMem)

							if !changed {
								if expectLive {
									t.Errorf("Liveness analysis failed - ran out of refinements")
								} else {
									t.Log("Ran out of refinements as expected")
								}

								break
							}

							break
						}
					}
				}
			})
		})

		nmgr.ForEach(func(i int, note *expect.Note) {
			if !foundMatch[i] {
				t.Fatalf("Program had expect note that was not found. %d %v %v\n", i, note, fset.Position(note.Pos))
			}
		}
	*/
}

func runTestsOnPackages(t *testing.T, pkgs []string, short bool, nameFunc func(string) string) {
	if testing.Short() && !short {
		t.Skip("Skipping test in -short mode")
	}

	for _, pkg := range pkgs {
		testName := pkg
		if nameFunc != nil {
			testName = nameFunc(testName)
		}

		t.Run(testName, func(t *testing.T) {
			t.Log("Testing", pkg)
			analyzePackage(t, pkg)
		})
	}
}

// NOTE: Tests are disabled because they are (currently) superseded by suites in the
// absint package that run the static analysis on the same pieces of code.
func testSimple(t *testing.T) {
	testPackages := tu.ListSimpleTests(t, "..")

	runTestsOnPackages(t, testPackages, true, func(name string) string {
		return strings.TrimPrefix(name, "simple-examples/")
	})
}

func testNoSelect(t *testing.T) {
	// Tests with communication without select.
	// List compiled with:
	// rg --glob '**/*.go' --files-without-match select | xargs rg --files-with-matches "<-" | xargs dirname | sort
	testPackages := strings.Split(`
adv-go-pat/ping-pong
channels
channel-scoping-test
commaok
go-patterns/confinement/buffered-channel
go-patterns/generator
hello-world
issue-11-non-communicating-fn-call
loop-variations
makechan-in-loop
method-test
multi-makechan-same-var
nested
producer-consumer
semaphores
send-recv-with-interfaces
session-types-benchmarks/branch-dependent-deadlock
session-types-benchmarks/deadlocking-philosophers
session-types-benchmarks/fixed
session-types-benchmarks/giachino-concur14-factorial
session-types-benchmarks/github-golang-go-issue-12734
session-types-benchmarks/parallel-buffered-recursive-fibonacci
session-types-benchmarks/parallel-recursive-fibonacci
session-types-benchmarks/parallel-twoprocess-fibonacci
session-types-benchmarks/philo
session-types-benchmarks/popl17-fact
session-types-benchmarks/popl17-fib
session-types-benchmarks/popl17-fib-async
session-types-benchmarks/popl17-mismatch
session-types-benchmarks/popl17-sieve
session-types-benchmarks/ring-pattern
session-types-benchmarks/russ-cox-fizzbuzz
session-types-benchmarks/spawn-in-choice
session-types-benchmarks/squaring-fanin
session-types-benchmarks/squaring-fanin-bad
session-types-benchmarks/squaring-pipeline
simple
single-gortn-method-call
struct-done-channel`, "\n")[1:]

	runTestsOnPackages(t, testPackages, true, nil)
}

func testWithSelect(t *testing.T) {
	// Tests with communications with select.
	// List compiled with:
	// rg --glob '**/*.go' --files-with-matches select | xargs dirname | sort
	// session-types-benchmarks/powsers - Takes a long time to complete
	testPackages := strings.Split(`
fanin-pattern-commaok
fcall
go-patterns/bounded
go-patterns/parallel
go-patterns/semaphore
liveness-bug
md5
multiple-timeout
popl17ae/emptyselect
select-with-continuation
select-with-weak-mismatch
session-types-benchmarks/dining-philosophers
session-types-benchmarks/fanin-pattern
session-types-benchmarks/giachino-concur14-dining-philosopher
session-types-benchmarks/popl17-alt-bit
session-types-benchmarks/popl17-concsys
session-types-benchmarks/popl17-cond-recur
session-types-benchmarks/popl17-fanin
session-types-benchmarks/popl17-fanin-alt
session-types-benchmarks/popl17-forselect
session-types-benchmarks/popl17-jobsched
session-types-benchmarks/squaring-cancellation
timeout-behaviour
wiki`, "\n")[1:]

	runTestsOnPackages(t, testPackages, true, nil)
}

func testGoKer(t *testing.T) {
	runTestsOnPackages(t, tu.ListGoKerPackages(t, ".."), false, tu.GoKerTestName)
}
