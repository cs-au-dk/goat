package pkgutil

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/tools/go/packages"
)

// LoadConfig is a structure according to which Go pacakge loading is configured.
// It loads a package in module-aware mode, or GOPATH mode based on how the .GoPath
// and .ModulePath fields are set. If IncludeTests is true, package loading will also
// expose test functions.
type LoadConfig struct {
	GoPath, ModulePath string
	IncludeTests       bool
}

// loadMode avoids deprecation warnings from using packages.LoadAllSyntax.
// It sets all packages.Need* options.
const loadMode packages.LoadMode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
	packages.NeedImports | packages.NeedTypes | packages.NeedTypesSizes | packages.NeedSyntax |
	packages.NeedTypesInfo | packages.NeedDeps

var (
	// moduleRegex is a regular expression according to which a go.mod file can be parsed.
	moduleRegex = regexp.MustCompile(`(?m)^module\s+(.*)$`)

	// cwd retrieves the current working directory on Goat invocation.
	cwd = func() string {
		if dir, err := os.Getwd(); err == nil {
			return dir
		} else {
			panic(err)
		}
	}()
)

// relativizingParseFile is a ParseFile implementation that relativizes
// filenames according to CWD. This is an easy way to globally make paths
// system agnostic, which is useful for golden tests involving file paths.
// The alternative is to manually relativize paths at every location we
// print one, but that is made difficult by us relying on built-in
// implementations of String methods, which we would have to circumvent.
func relativizingParseFile(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
	if rel, err := filepath.Rel(cwd, filename); err == nil {
		filename = rel
	}
	const mode = parser.AllErrors | parser.ParseComments
	return parser.ParseFile(fset, filename, src, mode)
}

// LoadPackages loads the AST of the specified packaged according to the provided LoadConfig.
func LoadPackages(cfg LoadConfig, packageName string) ([]*packages.Package, error) {
	gopath, err := filepath.Abs(cfg.GoPath)
	if err != nil {
		return nil, err
	}

	config := &packages.Config{
		Mode:      loadMode,
		Tests:     cfg.IncludeTests,
		ParseFile: relativizingParseFile,
	}

	if modulePath := cfg.ModulePath; modulePath != "" {
		// Load packages according to the new "module-aware" mode (GO111MODULE=on).
		pkgPath, err := filepath.Abs(modulePath)
		if err != nil {
			return nil, err
		}

		contents, err := os.ReadFile(filepath.Join(pkgPath, "go.mod"))
		if err != nil {
			return nil, fmt.Errorf("Unable to load 'go.mod' file at %s.\n%w", modulePath, err)
		}

		m := moduleRegex.FindSubmatch(contents)
		if len(m) <= 1 {
			return nil, fmt.Errorf("Unable to locate module name in 'go.mod' file")
		}

		config.Dir = pkgPath
		config.Env = append(os.Environ(), "GOPATH="+gopath, "GO111MODULE=on")
	} else {
		// Load packages according to the legacy "module-unaware" mode (GO111MODULE=off).
		config.Env = append(os.Environ(), "GOPATH="+gopath, "GO111MODULE=off")
	}

	return loadPackagesWithConfig(config, packageName)
}

// LoadPackagesFromSource loads packages directly from strings as source files.
// It is mainly useful for testing.
func LoadPackagesFromSource(source string) ([]*packages.Package, error) {
	// We use the Overlay mechanism to allow the tool to load a non-existent file.
	config := &packages.Config{
		Mode:  loadMode,
		Tests: false,
		Dir:   "",
		Env:   append(os.Environ(), "GO111MODULE=off", "GOPATH=/fake"),
		Overlay: map[string][]byte{
			"/fake/testpackage/main.go": []byte(source),
		},
	}

	return loadPackagesWithConfig(config, "/fake/testpackage/main.go")
}

// loadPackagesWithConfig wraps around packages.Load, that loads the package specified
// by `query` according to the given configuration, and performs additional filtering when
// loading includes test packages.
func loadPackagesWithConfig(config *packages.Config, query string) ([]*packages.Package, error) {
	pkgs, err := packages.Load(config, query)
	if err != nil {
		return nil, err
	} else if packages.PrintErrors(pkgs) > 0 {
		return nil, errors.New("errors encountered while loading packages")
	}
	if config.Tests {
		// Deduplicate packages that have test functions (such packages are
		// returned twice, once with no tests and once with tests. We discard
		// the package without tests.) This prevents duplicate versions of the
		// same types, functions, ssa values, etc., which can be very confusing
		// when debugging.
		packageIDs := map[string]bool{}
		for _, pkg := range pkgs {
			packageIDs[pkg.ID] = true
		}

		filteredPkgs := []*packages.Package{}
		for _, pkg := range pkgs {
			if !packageIDs[fmt.Sprintf("%s [%s.test]", pkg.ID, pkg.ID)] {
				filteredPkgs = append(filteredPkgs, pkg)
			}
		}
		pkgs = filteredPkgs
	}
	return pkgs, nil
}
