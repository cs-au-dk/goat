package utils

import (
	"flag"
	"fmt"
	"log"
	"strings"
)

type options struct {
	goroBound       uint
	minlen          uint
	pseti           int
	nodesep         float64
	function        string
	outputFormat    string
	gopath          string
	modulePath      string
	psets           string
	task            string
	logai           bool
	metrics         bool
	noColorize      bool
	httpDebug       bool
	verbose         bool
	extended        bool
	packageSplit    bool
	includeInternal bool
	includeTests    bool
	localPackages   bool
	visualize       bool
	fullCg          bool
	justGoros       bool
	skipChanNames   bool
	skipSync        bool
	noAbort         bool
}

const (
	_STATIC_METRICS = iota
	_CAN_BUILD
	_GORO_TOPOLOGY
	_CYCLES
	_CHANNEL_ALIASING
	_CFG_TO_DOT
	_ABSTRACT_INTERP
	_ANALYZE
	_POINTS_TO
	_POSITION
	_WRITTEN_FIELDS_ANALYSIS
	_COLLECT_PRIMITIVES
	_CHECK_PSETS
)

const (
	_PSET_SINGLETON = iota
	_PSET_GCATCH
	_PSET_INTRA_DEP
	_PSET_TOTAL
	_PSET_SAMEFUNC
)

func CanColorize(col func(...interface{}) string) func(...interface{}) string {
	if opts.noColorize {
		return func(is ...interface{}) string {
			return fmt.Sprintf(strings.Repeat("%s", len(is)), is...)
		}
	}
	return col
}

var task = []struct{ flag, explanation string }{{
	"static-metrics",
	"Uses the upfront analysis to statically collect metrics on over-approximations, e. g. channel points-to set",
}, {
	"check-can-build",
	"Performs a mock building of the package, attempting pointer analysis and SSA construction",
}, {
	"goroutine-topology",
	"Construct the goroutine topology graph",
}, {
	"check-cycles",
	"Check for cycles on the goroutine topology graph",
}, {
	"check-channel-aliasing",
	"Check the size of the points-to set for each channel and report the maximum",
}, {
	"cfg-to-dot",
	"Create a graph for the control-flow graph",
}, {
	"abstract-interp",
	"Perform abstract interpretation",
}, {
	"analyze",
	"Perform semi-dynamic analysis",
}, {
	"points-to",
	"Perform points-to analysis and log all points-to sets",
}, {
	"positions",
	"Print all SSA functions found, and the position of each instruction",
}, {
	"written-fields",
	"Print the result of running the upfront written fields analysis",
}, {
	"collect-primitives",
	"Print the result of collecting all primitives in all the functions",
}, {
	"check-psets",
	"Print the result of computing Psets",
}}

var psets = []struct{ flag, explanation string }{{
	"singleton",
	"Primitive sets consist of singletons of channels, identified by allocation site",
}, {
	"gcatch",
	"Primitive sets are GCatch-style P-sets, (channels with intra-procedural mutual dependencies)",
}, {
	"intra-dependent",
	"Primitive sets are formed from channels of which the operations have intra-procedural control flow dependencies",
}, {
	"total",
	"Construct a single large P-set including all channels",
}, {
	"samefunc",
	"Primitive sets are formed by merging primitives that are allocated or used in the same function",
}}

var opts = &options{}

type optInterface struct{}

type taskInterface struct{}

type psetInterface struct{}

func Opts() optInterface {
	return optInterface{}
}

func (optInterface) NoColorize() bool {
	return opts.noColorize
}

func (optInterface) GoroBound() int {
	return int(opts.goroBound)
}

func (optInterface) PSetIndex() int {
	return opts.pseti
}

func (optInterface) IsPickedPset(i int) bool {
	return opts.pseti == -1 || opts.pseti == i
}

func (optInterface) WithinGoroBound(i int) bool {
	return i < int(opts.goroBound)
}

func (optInterface) ExceedsGoroBound(i int) bool {
	return int(opts.goroBound) < i
}

func (optInterface) Minlen() uint {
	return opts.minlen
}
func (optInterface) Nodesep() float64 {
	return opts.nodesep
}
func (optInterface) Function() string {
	return opts.function
}
func (optInterface) OutputFormat() string {
	return opts.outputFormat
}
func (optInterface) GoPath() string {
	return opts.gopath
}
func (optInterface) ModulePath() string {
	return opts.modulePath
}
func (optInterface) LogAI() bool {
	return opts.logai
}
func (optInterface) PSets() psetInterface {
	return psetInterface{}
}
func (psetInterface) Singleton() bool {
	return opts.psets == psets[_PSET_SINGLETON].flag
}
func (psetInterface) GCatch() bool {
	return opts.psets == psets[_PSET_GCATCH].flag
}
func (psetInterface) IntraDependent() bool {
	return opts.psets == psets[_PSET_INTRA_DEP].flag
}
func (psetInterface) Total() bool {
	return opts.psets == psets[_PSET_TOTAL].flag
}
func (psetInterface) SameFunc() bool {
	return opts.psets == psets[_PSET_SAMEFUNC].flag
}
func (optInterface) Task() taskInterface {
	return taskInterface{}
}
func (taskInterface) IsAbstractInterpretation() bool {
	return opts.task == task[_ABSTRACT_INTERP].flag
}
func (taskInterface) IsStaticMetrics() bool {
	return opts.task == task[_STATIC_METRICS].flag
}
func (taskInterface) IsCanBuild() bool {
	return opts.task == task[_CAN_BUILD].flag
}
func (taskInterface) IsCfgToDot() bool {
	return opts.task == task[_CFG_TO_DOT].flag
}
func (taskInterface) IsChannelAliasingCheck() bool {
	return opts.task == task[_CHANNEL_ALIASING].flag
}
func (taskInterface) IsGoroTopology() bool {
	return opts.task == task[_GORO_TOPOLOGY].flag
}
func (taskInterface) IsCycleCheck() bool {
	return opts.task == task[_CYCLES].flag
}
func (taskInterface) IsPointsTo() bool {
	return opts.task == task[_POINTS_TO].flag
}
func (taskInterface) IsWrittenFieldsAnalysis() bool {
	return opts.task == task[_WRITTEN_FIELDS_ANALYSIS].flag
}
func (taskInterface) IsCollectPrimitives() bool {
	return opts.task == task[_COLLECT_PRIMITIVES].flag
}
func (taskInterface) IsCheckPsets() bool {
	return opts.task == task[_CHECK_PSETS].flag
}
func (taskInterface) IsPosition() bool {
	return opts.task == task[_POSITION].flag
}
func (taskInterface) IsRuntimeAnalysis() bool {
	return opts.task == task[_ANALYZE].flag
}
func (optInterface) Metrics() bool {
	return opts.metrics
}
func (optInterface) HttpDebug() bool {
	return opts.httpDebug
}
func (optInterface) Verbose() bool {
	return opts.verbose
}
func (optInterface) Extended() bool {
	return opts.extended
}
func (optInterface) PackageSplit() bool {
	return opts.packageSplit
}
func (optInterface) IncludeInternal() bool {
	return opts.includeInternal
}
func (optInterface) IncludeTests() bool {
	return opts.includeTests
}
func (optInterface) LocalPackages() bool {
	return opts.localPackages
}
func (optInterface) Visualize() bool {
	return opts.visualize
}
func (optInterface) FullCg() bool {
	return opts.fullCg
}
func (optInterface) JustGoros() bool {
	return opts.justGoros
}
func (optInterface) SkipChanNames() bool {
	return opts.skipChanNames
}
func (optInterface) SkipSync() bool {
	return opts.skipSync
}
func (optInterface) NoAbort() bool {
	return opts.noAbort
}

func init() {
	taskFlag := "\n"
	for _, task := range task {
		taskFlag += task.flag + " -- " + task.explanation + "\n"
	}
	taskFlag += "\n"
	psetFlag := "\n"
	for _, pset := range psets {
		psetFlag += pset.flag + " -- " + pset.explanation + "\n"
	}
	psetFlag += "\n"

	flag.UintVar(&(opts.minlen), "minlen", 2, "Minimum edge length (for wider output).")
	flag.Float64Var(&(opts.nodesep), "nodesep", 0.35, "Minimum space between two adjacent nodes in the same rank (for taller output).")
	flag.IntVar(&(opts.pseti), "pset", -1, "Index of Pset to analyze.")
	flag.StringVar(&(opts.function), "fun", "main", "target a specific function w. r. t. the given task.\n"+
		"- Function names need not be fully qualified w.r.t. package name. If a simple name is provided, "+
		"the framework will search for a function matching that name in the main package. If one is not found, "+
		"it will proceed to do a search across all packages. Will return the first function matching that name.\n"+
		"- Use '.' to perform targetted analysis on all functions in the main package.\n")
	flag.StringVar(&(opts.outputFormat), "format", "svg", "output file format [svg | png | jpg | ...]")
	flag.StringVar(&(opts.gopath), "gopath", "examples", "specify GOPATH to be used for packages.Load")
	flag.StringVar(&(opts.modulePath), "modulepath", "", `specify a path to a directory containing a Go module.
- If provided this will make our code loading tools (that piggyback on Go's tools) run
in "module-aware" mode (GO111MODULE=on).`)
	flag.StringVar(&(opts.psets), "psets", psets[_PSET_SINGLETON].flag, "When collecting primitives, determine primitive grouping strategy. Options:"+psetFlag)
	flag.StringVar(&(opts.task), "task", task[_ABSTRACT_INTERP].flag, "Set the task to do during execution. Options:"+taskFlag)
	flag.BoolVar(&(opts.logai), "ai-logging", false, "Enable logging of specific events during abstract interpretation")
	flag.BoolVar(&(opts.metrics), "metrics", false, "Enable collection of performance metrics for abstract interpretation")
	flag.BoolVar(&(opts.noColorize), "no-colorize", false, "Disable pretty printer colorization")
	flag.BoolVar(&(opts.extended), "extended", false, "Include additional information, e. g. channel buffer size and closing.")
	flag.BoolVar(&(opts.verbose), "verbose", false, "enable verbose output")
	flag.BoolVar(&(opts.packageSplit), "pkg-split", false, "sequential cross-package calls get distinct nodes and edges in the graph.")
	flag.BoolVar(&(opts.localPackages), "local-pkgs", false, "focus only local packages; when set, -internal-go is implicitly set to false.")
	flag.BoolVar(&(opts.includeInternal), "internal-go", false, "internal GO package goroutines included in the graph.")
	flag.BoolVar(&(opts.includeTests), "include-tests", false, "include main package test files in the analysis.")
	flag.BoolVar(&(opts.justGoros), "just-goroutines", false, "channels are excluded from the graph.")
	flag.BoolVar(&(opts.fullCg), "full-cg", false, "disable goroutine transitive closure in callgraph")
	flag.BoolVar(&(opts.skipChanNames), "skip-chan-names", false, "disable associating channel allocation sites with source code given names in the original AST")
	flag.BoolVar(&(opts.visualize), "visualize", false, "enable visualization via XDot")
	flag.BoolVar(&(opts.skipSync), "skip-sync", false, "skip special modelling of features of the 'sync' library")
	flag.BoolVar(&(opts.noAbort), "no-abort", false, "disable aborts upon critical precision loss")
	flag.UintVar(&(opts.goroBound), "goro-bound", 1, "set upper bound for dynamically spawned goroutines")
	flag.BoolVar(&(opts.httpDebug), "http-debug", false, "Start an http/pprof server for debugging")

	// Set up logging
	log.SetFlags(log.Ltime | log.Lshortfile)
}

func ParseArgs() {
	// Calling flag.Parse in init messes up unit tests.
	// See https://stackoverflow.com/questions/60235896/flag-provided-but-not-defined-test-v
	flag.Parse()

	validTask := false
	for _, task := range task {
		if task.flag == opts.task {
			validTask = true
			break
		}
	}

	if !validTask {
		log.Fatalf("Value \"%s\" is not valid for -task", opts.task)
	}

	if opts.localPackages {
		opts.includeInternal = false
	}
	if Opts().Task().IsCfgToDot() {
		opts.noColorize = true
	}
	switch {
	case Opts().Task().IsCycleCheck():
		opts.justGoros = true
		opts.packageSplit = false
	case Opts().Task().IsChannelAliasingCheck():
		opts.justGoros = false
		opts.localPackages = false
	}
}

func (optInterface) AnalyzeAllFuncs() bool {
	return opts.function == "."
}

func (optInterface) IsWholeProgramAnalysis() bool {
	return (Opts().Task().IsAbstractInterpretation() ||
		Opts().Task().IsCfgToDot() ||
		Opts().Task().IsCollectPrimitives()) &&
		opts.function == "main"
}

func (taskInterface) IsWholeProgramAnalysis() bool {
	return Opts().IsWholeProgramAnalysis()
}

func (optInterface) OnVerbose(do func()) {
	if Opts().Verbose() {
		do()
	}
}
