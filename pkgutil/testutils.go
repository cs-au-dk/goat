package pkgutil

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"log"
	"strings"

	"golang.org/x/tools/go/ssa"
)

func TestFunctions(prog *ssa.Program) (res []*ssa.Function) {
	testingPkg := prog.ImportedPackage("testing")
	if testingPkg == nil {
		// testing package is not loaded so no tests are defined.
		return
	}

	arg0Type := types.NewPointer(testingPkg.Type("T").Type())

	for _, pkg := range AllPackages(prog) {
		for name, member := range pkg.Members {
			if fun, ok := member.(*ssa.Function); ok && strings.HasPrefix(name, "Test") &&
				len(fun.Params) == 1 && types.Identical(arg0Type, fun.Params[0].Type()) {

				res = append(res, fun)
			}
		}
	}

	return
}


// A type satisfying the types.Importer interface. It can only import the
// package it is initialized with and the testing package.
type fakeImporter types.Package

func (f *fakeImporter) Import(path string) (*types.Package, error) {
	p := (*types.Package)(f)
	if path == p.Path() {
		return p, nil
	} else if path == "testing" {
		for _, pkg := range p.Imports() {
			if pkg.Path() == path {
				return pkg, nil
			}
		}
	}
	log.Fatalln("Unexpected import of", path)
	return nil, nil
}


// Creates a fake main package that calls the supplied test function.
func CreateFakeTestMainPackage(testFun *ssa.Function) *ssa.Package {
	testPkg := testFun.Pkg.Pkg
	prog := testFun.Prog

	file, err := parser.ParseFile(
		prog.Fset,
		"main.go",
		fmt.Sprintf(`package main
		import (
			pkg "%s"
			"testing"
		)

		func main() {
			var t testing.T
			pkg.%s(&t)
		}`, testPkg.Path(), testFun.Name()),
		0,
	)

	if err != nil {
		log.Fatal(err)
	}

	files := []*ast.File{file}

	pkg := types.NewPackage(testPkg.Path()+".synth", "main")
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}

	if err := types.NewChecker(
		&types.Config{Importer: (*fakeImporter)(testPkg)},
		prog.Fset, pkg, info,
	).Files(files); err != nil {
		log.Fatalln(err)
	}

	spkg := prog.CreatePackage(pkg, files, info, false)
	spkg.Build()
	return spkg
}
