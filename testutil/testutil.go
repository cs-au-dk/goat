package testutil

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/cs-au-dk/goat/analysis/cfg"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/analysis/upfront/condinline"
	"github.com/cs-au-dk/goat/analysis/upfront/loopinline"
	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/utils/graph"

	_ "github.com/fatih/color"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// LoadResult contains relevant information obtained after loading a Go program.
// It includes the SSA representation of the program, the control-flow graph,
// and the results of the pre-analysis (points-to analysis, side-effect analysis, etc.).
type LoadResult struct {
	// MainPkg is the package focused by the analysis.
	MainPkg *packages.Package
	// Prog is the SSA representation of the entire program.
	Prog *ssa.Program
	// Mains denotes all the packages that can act as entry points.
	Mains []*ssa.Package
	// Cfg is the the specialized control-flow graph for a given Go program.
	Cfg        *cfg.Cfg
	Goros      u.GoTopology
	GoroCycles u.GoCycles
	// Pointer represnts the result of the points-to analysis.
	Pointer *u.PointerResult
	// CallDAG encodes the SCC decomposition of the complete program call graph.
	CallDAG graph.SCCDecomposition[*ssa.Function]
	// PrunedCallDAG encodes the SCC decomposition of the pruned program call graph.
	PrunedCallDAG graph.SCCDecomposition[*ssa.Function]
	// CtrLocPriorities maps the priorities of each control location
	CtrLocPriorities u.CtrLocPriorities
	// WrittenFields contains the result of the side-effect analysis.
	WrittenFields u.WrittenFields
}

// upfrontAnalyses computes the collection of local packages, and populates the LoadResult
// with the results of the pre-analysis:
//   - The results of the points-to analysis.
//   - The enhanced CFG.
//   - The (pruned) call graph, and its SCC condensation as a call DAG.
//   - The results of the side-effect analysis.
//   - The priorities of control locations.
func (res *LoadResult) upfrontAnalyses() {
	pkgutil.GetLocalPackages(res.Mains, res.Prog.AllPackages())

	// If the points-to analysis information is not ready, compute it.
	if res.Pointer == nil {
		res.Pointer = u.TotalAndersen(res.Prog, res.Mains)
	}

	// If the CFG is not ready, compute it.
	if res.Cfg == nil {
		res.Cfg = cfg.GetCFG(res.Prog, res.Mains, &res.Pointer.Result)
	}

	// Compute the call graph SCC condensation (and the pruned variant).
	cg := res.Pointer.CallGraph
	entries := []*ssa.Function{cg.Root.Func}
	res.CallDAG = graph.FromCallGraph(cg, false).SCC(entries)
	res.PrunedCallDAG = graph.FromCallGraph(cg, true).SCC(entries)

	// Compute control location priorities, based on the pruned call DAG.
	res.CtrLocPriorities = u.GetCtrLocPriorities(res.Cfg.Functions(), res.PrunedCallDAG)
	res.WrittenFields = u.ComputeWrittenFields(res.Pointer, res.PrunedCallDAG)
}

// LoadExampleAsPackages loads an example package to be used for a test.
func LoadExampleAsPackages(t *testing.T, pathToRoot string, pkg string, loopInline bool) []*packages.Package {
	// Invoking the package tools is slow because it uses `go list` under the hood.
	// If the package doesn't have imports we can take a fast path by loading the
	// code manually and parsing it ourselves.
	srcDir := pathToRoot + "/examples/src/" + pkg
	if entries, err := os.ReadDir(srcDir); err == nil {
		if len(entries) == 1 {
			entry := entries[0]
			if !entry.IsDir() && entry.Name() == "main.go" {
				if content, err := os.ReadFile(srcDir + "/main.go"); err == nil &&
					// Assert no imports
					!bytes.Contains(content, []byte("import")) {
					return LoadSourceAsPackages(t, pkg, string(content))
				}
			}
		}
	}

	// Create a main-package from the specified package
	pkgs, err := pkgutil.LoadPackages(pkgutil.LoadConfig{GoPath: pathToRoot + "/examples"}, pkg)
	if err != nil {
		t.Fatal(err)
	}

	if len(pkgs) != 1 {
		t.Fatal("Example contains more than just a main package?")
	}

	if err := u.ASTTranform(pkgs, condinline.Transform); err != nil {
		t.Fatal("Cond inlining failed:", err)
	}

	if loopInline {
		if err := u.ASTTranform(pkgs, loopinline.Transform); err != nil {
			t.Fatal("Loop inlining failed:", err)
		}
	}
	return pkgs
}

func LoadExamplePackage(t *testing.T, pathToRoot string, pkg string) LoadResult {
	return loadExamplePackage(t, pathToRoot, pkg, false)
}

func LoadLoopInlinedExamplePackage(t *testing.T, pathToRoot string, pkg string) LoadResult {
	return loadExamplePackage(t, pathToRoot, pkg, true)
}

func loadExamplePackage(t *testing.T, pathToRoot string, pkg string, loopInline bool) LoadResult {
	return LoadResultFromPackages(t, LoadExampleAsPackages(t, pathToRoot, pkg, loopInline))
}

func LoadResultFromPackages(t *testing.T, pkgs []*packages.Package) (res LoadResult) {
	mainpkg := pkgs[0]
	res.MainPkg = mainpkg

	u.CollectNames(pkgs)

	res.Prog, _ = ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions|ssa.InstantiateGenerics)
	res.Prog.Build()

	res.Mains = ssautil.MainPackages(res.Prog.AllPackages())

	res.upfrontAnalyses()

	return
}

func LoadSourceAsPackages(t *testing.T, importPath string, content string) []*packages.Package {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(
		fset,
		"main.go",
		content,
		parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	files := []*ast.File{file}

	// First argument is package path, the second is name.
	pkg := types.NewPackage(importPath, "main")
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Instances:  make(map[*ast.Ident]types.Instance),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	if err := types.NewChecker(
		&types.Config{Importer: importer.Default()},
		fset, pkg, info).Files(files); err != nil {
		t.Fatal(err)
	}

	// If the package does not have imports we can take a fast path.
	if len(pkg.Imports()) == 0 {
		return []*packages.Package{{
			// ID is a unique identifier for a package,
			// in a syntax provided by the underlying build system.
			//
			// Because the syntax varies based on the build system,
			// clients should treat IDs as opaque and not attempt to
			// interpret them.
			ID: "pkg-loaded-from-src",

			// Name is the package name as it appears in the package source code.
			Name: pkg.Name(),

			// PkgPath is the package path as used by the go/types package.
			PkgPath: pkg.Path(),

			// Types provides type information for the package.
			// The NeedTypes LoadMode bit sets this field for packages matching the
			// patterns; type information for dependencies may be missing or incomplete,
			// unless NeedDeps and NeedImports are also set.
			Types: pkg,

			// Fset provides position information for Types, TypesInfo, and Syntax.
			// It is set only when Types is set.
			Fset: fset,

			// Syntax is the package's syntax trees, for the files listed in CompiledGoFiles.
			//
			// The NeedSyntax LoadMode bit populates this field for packages matching the patterns.
			// If NeedDeps and NeedImports are also set, this field will also be populated
			// for dependencies.
			//
			// Syntax is kept in the same order as CompiledGoFiles, with the caveat that nils are
			// removed.  If parsing returned nil, Syntax may be shorter than CompiledGoFiles.
			Syntax: files,

			// TypesInfo provides type information about the package's syntax trees.
			// It is set only when Syntax is set.
			TypesInfo: info,
		}}
	}

	// Otherwise we need to invoke the packages tool that can import code for
	// dependencies. The reason to not just do this for all packages is that
	// it's a lot slower than the above because it needs to invoke the go tool
	// in a subprocess.
	pkgs, err := pkgutil.LoadPackagesFromSource(content)
	if err != nil {
		t.Fatal(err)
	}
	if err := u.ASTTranform(pkgs, condinline.Transform); err != nil {
		t.Fatal("Cond inlining failed:", err)
	}
	return pkgs
}

func LoadPackageFromSource(t *testing.T, importPath string, content string) (res LoadResult) {
	return LoadResultFromPackages(t, LoadSourceAsPackages(t, importPath, content))
}

var analysisLock sync.Mutex

// Utility function for running analysis tests in parallel. Expensive things
// such as loading code from disk, constructing SSA, performing pointer
// analysis, etc. is done in parallel.
// Call t.Parallel() before calling this function.
func ParallelHelper(t *testing.T, pkgs []*packages.Package, f func(LoadResult)) {
	var res LoadResult

	// Perform expensive pre-analyses in parallel
	res.MainPkg = pkgs[0]
	res.Prog, _ = ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions|ssa.InstantiateGenerics)
	res.Prog.Build()

	res.Mains = ssautil.MainPackages(res.Prog.AllPackages())

	res.Pointer = u.TotalAndersen(res.Prog, res.Mains)
	res.Cfg = cfg.GetCFG(res.Prog, res.Mains, &res.Pointer.Result)

	// Analyses or procedures that touch global state are protected by the
	// analysisLock. This includes CollectNames, GetLocalPackages and the main
	// test function (which we assume will perform a static analysis run).
	analysisLock.Lock()
	defer analysisLock.Unlock()

	t.Logf("Running %v", t.Name())

	u.CollectNames(pkgs)
	res.upfrontAnalyses()

	f(res)
}

func ListGoKerPackages(t *testing.T, pathToRoot string) []string {
	path := filepath.Join(pathToRoot, "examples/src")
	gokerPath := "gobench/goker"
	blacklist := map[string]map[string][]string{
		"blocking": {
			"grpc": {
				"862", // Heavy use of context.WithCancel that times out the analysis
				"862_fixed",
			},
			"hugo": {
				"5379", // sync.Once - too many standard libraries
				// Spurious cycles cause analysis blowup
				// Analysis of syscall.Getenv crashes due to syscall.envs
				// having an under-approximated points-to set (because the
				// pointer analysis does not try to reason about foreign code).
			},
			"etcd": {
				// Needs investigation after introduction of IndexAddr work-around.
				// Possibly fixable by implementing precise model for `len` on nil.
				"7443",
			},
		},
		"nonblocking": {
			"grpc": {
				"1687", // Testing variable used.
			},
			"istio": {
				"8144", // Use of sync.Map
			},
			"serving": {
				"3068", // Testing variable used.
				"4908", // Testing variable used.
				"6171", // Testing variable used.
			},
		},
	}

	filter := func(strs []string, blacklist []string) (res []string) {
	STRING_LOOP:
		for _, str := range strs {
			for _, blstr := range blacklist {
				if str == blstr {
					continue STRING_LOOP
				}
			}
			res = append(res, str)
		}
		return
	}

	makeLisa := func(dir string) *exec.Cmd {
		return exec.Command("ls", "-a", filepath.Join(path, gokerPath, dir))
	}

	getLisaPaths := func(path string) []string {
		lsla := makeLisa(path)
		out, err := lsla.Output()
		if err != nil {
			panic(err)
		}
		parts := strings.Split(string(out), "\n")
		return parts[2 : len(parts)-1]
	}

	packages := []string{}

	processRepo := func(category string, repo string) {
		issues := getLisaPaths(filepath.Join(category, repo))
		issues = filter(issues, blacklist[category][repo])

		for _, issue := range issues {
			if issue != "" {
				packages = append(packages, filepath.Join(gokerPath, category, repo, issue))
			}
		}
	}

	for _, repo := range getLisaPaths("blocking") {
		processRepo("blocking", repo)
	}

	for _, repo := range getLisaPaths("nonblocking") {
		processRepo("nonblocking", repo)
	}

	return packages
}

func ListPackagesIn(t *testing.T, pathToRoot string, blacklist []string, bmDir string) []string {
	path := filepath.Join(pathToRoot, "examples/src")

	filter := func(strs []string, blacklist []string) (res []string) {
	STRING_LOOP:
		for _, str := range strs {
			for _, blstr := range blacklist {
				if str == blstr {
					continue STRING_LOOP
				}
			}
			res = append(res, str)
		}
		return
	}

	lsla := exec.Command("ls", "-a", filepath.Join(path, bmDir))

	getLisaPaths := func() []string {
		out, err := lsla.Output()
		if err != nil {
			panic(err)
		}
		parts := strings.Split(string(out), "\n")
		return parts[2 : len(parts)-1]
	}

	packages := []string{}

	processRepo := func() {
		benchmarks := getLisaPaths()
		benchmarks = filter(benchmarks, blacklist)

		for _, benchmark := range benchmarks {
			if benchmark != "" {
				packages = append(packages, filepath.Join(bmDir, benchmark))
			}
		}
	}

	processRepo()

	return packages
}

func RecListPackagesIn(t *testing.T, pathToRoot string, blacklist []string, bmDir string) []string {
	// fsys := fs.

	isMainPkg := func(FS fs.FS, name string) bool {
		ds, err := fs.ReadDir(FS, name)
		if err != nil {
			return false
		}

		count := 0
		for _, d := range ds {
			if d.IsDir() {
				count++
			}
		}

		return count == 0
	}

	path := filepath.Join(pathToRoot, "examples/src")

	filter := func(strs []string, blacklist []string) (res []string) {
	STRING_LOOP:
		for _, str := range strs {
			for _, blstr := range blacklist {
				if str == blstr {
					continue STRING_LOOP
				}
			}
			res = append(res, str)
		}
		return
	}

	packages := []string{}

	var processRepo func(string)
	processRepo = func(bmDir string) {
		lsla := exec.Command("ls", "-a", filepath.Join(path, bmDir))

		getLisaPaths := func() []string {
			out, err := lsla.Output()
			if err != nil {
				panic(err)
			}
			parts := strings.Split(string(out), "\n")
			return parts[2 : len(parts)-1]
		}

		benchmarks := getLisaPaths()
		benchmarks = filter(benchmarks, blacklist)

		for _, benchmark := range benchmarks {
			if benchmark != "" {
				if isMainPkg(os.DirFS(filepath.Join(path, bmDir)), benchmark) {
					packages = append(packages, filepath.Join(bmDir, benchmark))
				} else {
					processRepo(filepath.Join(bmDir, benchmark))
				}
			}
		}
	}

	processRepo(bmDir)

	return packages
}

func ListSessionTypesPackages(t *testing.T, pathToRoot string, blacklist []string) []string {
	return ListPackagesIn(t, pathToRoot, blacklist, "session-types-benchmarks")
}

func ListSyncPkgTests(t *testing.T, pathToRoot string, blacklist []string) []string {
	return RecListPackagesIn(t, pathToRoot, blacklist, "sync-pkg")
}

func ListSyncPkgCondTests(t *testing.T, pathToRoot string, blacklist []string) []string {
	return ListPackagesIn(t, pathToRoot, blacklist, "sync-pkg-cond")
}

func ListTopPointerTests(t *testing.T, pathToRoot string, blacklist []string) []string {
	return RecListPackagesIn(t, pathToRoot, blacklist, "top-pointers")
}

func ListPorPkgTests(t *testing.T, pathToRoot string, blacklist []string) []string {
	return ListPackagesIn(t, pathToRoot, blacklist, "por-examples")
}

func GoKerTestName(test string) string {
	strs := strings.Split(test, string(filepath.Separator))
	return strs[len(strs)-2] + string(filepath.Separator) + strs[len(strs)-1]
}

func ListSimpleTests(t *testing.T, pathToRoot string) []string {
	testPackages := []string{
		"micro",
	}

	fullPath := filepath.Join(pathToRoot, "examples/src/simple-examples")

	// Add all simple examples
	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, e error) error {
		if e != nil {
			return e
		}

		if info.Mode().IsRegular() && strings.HasSuffix(path, ".go") {
			// Check if the file has a main function
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			if strings.Contains(string(content), "func main()") {
				parts := strings.SplitN(filepath.Dir(path), "examples/src/", 2)
				testPackages = append(testPackages, parts[1])
			}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return testPackages
}

func ListAllTests(t *testing.T, pathToRoot string) []string {
	allTests := append(
		ListSimpleTests(t, pathToRoot),
		ListGoKerPackages(t, pathToRoot)...,
	)

	for _, f := range []func(*testing.T, string, []string) []string{
		ListSessionTypesPackages,
		ListSyncPkgTests,
		ListSyncPkgCondTests,
		ListTopPointerTests,
		ListPorPkgTests,
	} {
		allTests = append(allTests, f(t, pathToRoot, nil)...)
	}

	return allTests
}
