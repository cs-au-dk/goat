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
	"testing"

	"github.com/cs-au-dk/goat/analysis/cfg"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/utils/graph"

	_ "github.com/fatih/color"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func init() {
	//color.NoColor = false
}

type LoadResult struct {
	MainPkg          *packages.Package
	Prog             *ssa.Program
	Mains            []*ssa.Package
	Cfg              *cfg.Cfg
	Goros            u.GoTopology
	GoroCycles       u.GoCycles
	Pointer          *pointer.Result
	CallDAG          graph.SCCDecomposition[*ssa.Function]
	CtrLocPriorities u.CtrLocPriorities
	WrittenFields    u.WrittenFields
}

func (res *LoadResult) upfrontAnalyses() {
	pkgutil.GetLocalPackages(res.Mains, res.Prog.AllPackages())

	res.Pointer = u.TotalAndersen(res.Prog, res.Mains)

	res.Cfg = cfg.GetCFG(res.Prog, res.Mains, res.Pointer)
	// TODO: Revisit
	// if !utils.Opts().IsWholeProgramAnalysis() {
	// 	res.Goros = u.CollectGoros(res.Pointer)
	// 	res.GoroCycles = res.Goros.Cycles()
	// }

	cg := res.Pointer.CallGraph
	G := graph.FromCallGraph(cg, true)
	res.CallDAG = G.SCC([]*ssa.Function{cg.Root.Func})

	res.CtrLocPriorities = u.GetCtrLocPriorities(res.Cfg.Functions(), res.CallDAG)
	res.WrittenFields = u.ComputeWrittenFields(res.Pointer, res.CallDAG)
}

func LoadExamplePackage(t *testing.T, pathToRoot string, pkg string) LoadResult {
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
					return LoadPackageFromSource(t, pkg, string(content))
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

	return LoadResultFromPackages(t, pkgs)
}

func LoadResultFromPackages(t *testing.T, pkgs []*packages.Package) (res LoadResult) {
	mainpkg := pkgs[0]
	res.MainPkg = mainpkg

	u.CollectNames(pkgs)

	res.Prog, _ = ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions)
	res.Prog.Build()

	res.Mains = ssautil.MainPackages(res.Prog.AllPackages())

	res.upfrontAnalyses()

	return
}

func LoadPackageFromSource(t *testing.T, importPath string, content string) (res LoadResult) {
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
	spkg, _, err := ssautil.BuildPackage(
		&types.Config{Importer: importer.Default()},
		fset, pkg, files,
		ssa.SanityCheckFunctions,
	)
	if err != nil {
		t.Fatal(err)
	}

	// If the package does not have imports we can take a fast path.
	if len(pkg.Imports()) == 0 {
		spkg.Prog.Build()
		res.Prog = spkg.Prog
		res.Mains = []*ssa.Package{spkg}
		res.MainPkg = &packages.Package{
			Syntax: files,
		}

		res.upfrontAnalyses()

		return
	}

	// Otherwise we need to invoke the packages tool that can import code for
	// dependencies. The reason to not just do this for all packages is that
	// it's a lot slower than the above because it needs to invoke the go tool
	// in a subprocess.
	pkgs, err := pkgutil.LoadPackagesFromSource(content)
	if err != nil {
		t.Fatal(err)
	}
	return LoadResultFromPackages(t, pkgs)
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
				"5379", // SO - too many standard libraries
				// Spurious cycles cause analysis blowup
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
