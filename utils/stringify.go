package utils

import (
	"fmt"

	"github.com/fatih/color"

	"golang.org/x/tools/go/ssa"
)

const (
	// SHARED_PKG is a placeholder value for a package of which the name is missing.
	SHARED_PKG = "!#shared_pkg"
	// SHARED_FUN is a placeholder value for a function of which the name is missing.
	SHARED_FUN = "!#shared_fun"
)

// Colorization API
var (
	pkgColor = func(is ...interface{}) string {
		return CanColorize(color.New(color.FgBlue).SprintFunc())(is...)
	}
	funColor = func(is ...interface{}) string {
		return CanColorize(color.New(color.FgHiYellow).SprintFunc())(is...)
	}
	blkColor = func(is ...interface{}) string {
		return CanColorize(color.New(color.FgHiCyan).SprintFunc())(is...)
	}
	nameColor = func(is ...interface{}) string {
		return CanColorize(color.New(color.FgHiGreen).SprintFunc())(is...)
	}
	insColor = func(is ...interface{}) string {
		return CanColorize(color.New(color.FgHiWhite, color.Faint).SprintFunc())(is...)
	}
)

// SSAPkgString creates a pretty-printable package name.
func SSAPkgString(pkg *ssa.Package) (str string) {
	if pkg != nil {
		return pkgColor(pkg.Pkg.Path())
	}
	return pkgColor(SHARED_PKG)
}

// SSAValString creates a pretty-printable string from an SSA fimctopm.
func SSAFunString(fun *ssa.Function) string {
	if fun != nil {
		// TODO: Fix this mess by fixing the underlying problem of duplicate packages.
		// SSAFunString is used to uniquely identify functions, therefore the
		// output should be unique for functions with different pointer values.
		// In the case of programs loaded with tests, we can get multiple
		// copies of the same package in a program, which results in functions
		// that have identical strings but different pointer values.
		// The current work-around is to include the pointer-value in the string.
		return fmt.Sprintf("%p %s", fun, funColor(fun.String()))
	} else {
		return pkgColor(SHARED_PKG) + ":" + funColor(SHARED_FUN)
	}
}

// SSAValString creates a pretty-printable string from an SSA block.
func SSABlockString(blk *ssa.BasicBlock) string {
	if blk != nil {
		return SSAFunString(blk.Parent()) + ":" + blkColor(fmt.Sprintf("%d", blk.Index))
	}
	return pkgColor(SHARED_PKG) + ":" + funColor(SHARED_FUN)
}

// SSAValString creates a pretty-printable string from an SSA value.
func SSAValString(v ssa.Value) string {
	var name string
	if v != nil {
		name = v.Name() + " "
	}
	switch v := v.(type) {
	case ssa.Instruction:
		return SSABlockString(v.Block()) + ": " + nameColor(name) + "= " + insColor(v.String())
	default:
		if v == nil {
			return ""
		}
		return SSAFunString(v.Parent()) + ": " + insColor(v.String())
	}
}
