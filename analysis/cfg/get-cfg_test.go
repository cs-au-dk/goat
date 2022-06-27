package cfg_test

import (
	"Goat/testutil"
	"runtime/debug"
	"strings"
	"testing"
)

func cfgTestPackage(t *testing.T, pkg string) {
	defer func() {
		if err := recover(); err != nil {
			t.Errorf("Panic while analyzing...\n%v\n%s\n", err, debug.Stack())
		}
	}()

	testutil.LoadExamplePackage(t, "../..", pkg)
}

func runCfgTestsOnPackages(t *testing.T, pkgs []string) {
	if testing.Short() {
		t.Skip("CFG tests skipped in -short mode as they are implicitly run with other tests")
	}

	for _, pkg := range pkgs {
		// Hacky way to reduce length of simple-examples test names
		testName := strings.TrimPrefix(pkg, "simple-examples/")

		t.Run(testName, func(t *testing.T) {
			t.Log("Testing", pkg)
			cfgTestPackage(t, pkg)
		})
	}
}

func TestCfgSimple(t *testing.T) {
	runCfgTestsOnPackages(t, testutil.ListSimpleTests(t, "../.."))
}

func TestCfgNoSelect(t *testing.T) {
	// Tests with communication without select.
	// List compiled with:
	// rg --glob '**/*.go' --files-without-match select | xargs rg --files-with-matches "<-" | xargs dirname
	testPackages := strings.Split(`
adv-go-pat/ping-pong
channels
channel-scoping-test
commaok
go-patterns/confinement/buffered-channel
go-patterns/generator
hello-world
issue-11-non-communicating-fn-call
local-funcs/simple
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

	runCfgTestsOnPackages(t, testPackages)
}

func TestCfgWithSelect(t *testing.T) {
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
session-types-benchmarks/powsers
session-types-benchmarks/squaring-cancellation
timeout-behaviour
wiki`, "\n")[1:]

	runCfgTestsOnPackages(t, testPackages)
}

func TestCfgGoKer(t *testing.T) {
	runCfgTestsOnPackages(t, testutil.ListGoKerPackages(t, "../.."))
}
