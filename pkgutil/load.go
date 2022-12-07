package pkgutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"golang.org/x/tools/go/packages"
)

type LoadConfig struct {
	GoPath, ModulePath string
	IncludeTests       bool
}

var moduleRegex = regexp.MustCompile(`(?m)^module\s+(.*)$`)

// Load the AST for the packages matching the specified package name according
// to the provided LoadConfig.
func LoadPackages(cfg LoadConfig, packageName string) ([]*packages.Package, error) {
	gopath, err := filepath.Abs(cfg.GoPath)
	if err != nil {
		return nil, err
	}

	config := &packages.Config{
		Mode:  packages.LoadAllSyntax,
		Tests: cfg.IncludeTests,
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

	return LoadPackagesWithConfig(config, packageName)
}

// Mainly useful for testing
func LoadPackagesFromSource(source string) ([]*packages.Package, error) {
	// We use the Overlay mechanism to allow the tool to load a non-existent file.
	config := &packages.Config{
		Mode:  packages.LoadAllSyntax,
		Tests: false,
		Dir:   "",
		Env:   append(os.Environ(), "GO111MODULE=off", "GOPATH=/fake"),
		Overlay: map[string][]byte{
			"/fake/testpackage/main.go": []byte(source),
		},
	}

	return LoadPackagesWithConfig(config, "/fake/testpackage/main.go")
}

func LoadPackagesWithConfig(config *packages.Config, query string) ([]*packages.Package, error) {
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
