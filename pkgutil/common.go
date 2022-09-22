package pkgutil

import (
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

var opts = utils.Opts()

func CheckPkgInGoroot(pkg *types.Package) bool {
	path := filepath.Join(runtime.GOROOT(), "src", pkg.Path())
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		return true
	}
	return false
}

// Returns true iff. the function is in a package in the GOROOT.
func CheckInGoroot(fun *ssa.Function) bool {
	// var pkg *build.Package
	return fun != nil && fun.Pkg != nil &&
		CheckPkgInGoroot(fun.Pkg.Pkg)
}

// Determine what is the main package as follows:
// 1. Take the package with the most members
// 2. Skip the package suffixed with .test
func GetMain(mains []*ssa.Package) (main *ssa.Package) {
	for _, mp := range mains {
		if strings.HasSuffix(mp.String(), ".test") {
			continue
		}
		if main == nil || len(main.Members) < len(mp.Members) {
			main = mp
		}
	}
	return
}

func AllPackages(prog *ssa.Program) []*ssa.Package {
	mp := make(map[string]*ssa.Package)

	for _, pkg := range prog.AllPackages() {
		if strings.HasSuffix(pkg.String(), ".test") {
			continue
		}

		opkg, ok := mp[pkg.String()]
		if !ok || len(pkg.Members) > len(opkg.Members) {
			mp[pkg.String()] = pkg
		}
	}

	res := make([]*ssa.Package, 0, len(mp))
	for _, pkg := range mp {
		res = append(res, pkg)
	}

	return res
}
