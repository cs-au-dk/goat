package upfront

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/types"
	"os"

	"github.com/cs-au-dk/goat/pkgutil"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/packages"
)

type (
	// importer wraps around packages and can re-import packages
	// from the original run.
	importer []*types.Package

	// ASTTransformer generates an AST transformation function.
	ASTTransformer[T any] func(T) func(*astutil.Cursor) bool
)

var _ types.Importer = importer(nil)

// Import retrieves the package at the given string.
func (fi importer) Import(path string) (*types.Package, error) {
	for _, pkg := range fi {
		if pkg.Path() == path {
			return pkg, nil
		}
	}
	return nil, fmt.Errorf("Cannot import %s %v?", path, fi)
}

// ASTTransform applies the AST transformation function, including the current type
// information, to each file in the specified packages. Type information is
// re-computed when the transformation is complete.
func ASTTranform(
	pkgs []*packages.Package,
	transform ASTTransformer[map[ast.Expr]types.TypeAndValue],
) (rerr error) {
	newPkgs := map[*types.Package]*types.Package{}
	packages.Visit(pkgs, func(pkg *packages.Package) bool {
		if pkgutil.CheckPkgInGoroot(pkg.Types) {
			return false
		}

		tpkg := types.NewPackage(pkg.Types.Path(), pkg.Types.Name())
		newPkgs[pkg.Types] = tpkg
		return true
	}, func(pkg *packages.Package) {
		if pkgutil.CheckPkgInGoroot(pkg.Types) {
			return
		}

		transformer := transform(pkg.TypesInfo.Types)
		for i, f := range pkg.Syntax {
			pkg.Syntax[i] = astutil.Apply(f, nil, transformer).(*ast.File)
		}

		// Recompute type information on new AST
		// Mirrors the way type information is computed when package is initially
		// loaded in go/packages/packages.go:loadPackage
		info := &types.Info{
			Types:     make(map[ast.Expr]types.TypeAndValue),
			Defs:      make(map[*ast.Ident]types.Object),
			Uses:      make(map[*ast.Ident]types.Object),
			Implicits: make(map[ast.Node]types.Object),
			// loadPackage calls a function to initialize this field (?), but
			// it is in an internal package, so we just do it directly.
			Instances:  make(map[*ast.Ident]types.Instance),
			Scopes:     make(map[ast.Node]*types.Scope),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
		}

		tpkg := newPkgs[pkg.Types]
		// Replace imported packages with transformed versions (if applicable)
		imps := pkg.Types.Imports()
		for i, tpkg := range imps {
			if npkg, found := newPkgs[tpkg]; found {
				imps[i] = npkg
			}
		}

		if err := types.NewChecker(
			&types.Config{Importer: importer(imps)},
			pkg.Fset, tpkg, info,
		).Files(pkg.Syntax); err != nil {
			// Keep only the first error
			if rerr == nil {
				rerr = err
				for _, file := range pkg.Syntax {
					printer.Fprint(os.Stdout, pkg.Fset, file)
				}
			}
		}

		pkg.Types = tpkg
		pkg.TypesInfo = info
	})

	return
}
