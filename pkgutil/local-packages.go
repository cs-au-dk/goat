package pkgutil

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/tools/go/ssa"
)

var LocalPkgs map[*ssa.Package]bool

func pkgQualifiedPath(pkg *ssa.Package) []string {
	path := strings.Split(strings.TrimSuffix(pkg.Pkg.Path(), ".test"), "/")

	if path[0] == "vendor" {
		path = path[1:]
	}

	return path
}

/** GetLocalPackages */
func GetLocalPackages(mains []*ssa.Package, pkgs []*ssa.Package) (err error) {
	if len(mains) == 0 {
		return errors.New("gather local packages error: no main packages found")
	}

	LocalPkgs = make(map[*ssa.Package]bool)
	mp := GetMain(mains)
	if mp == nil {
		// If there is no non-test main package, just pick one of the test
		// packages.
		mp = mains[0]
	}

	mainpath := pkgQualifiedPath(mp)

	for _, p := range pkgs {
		pkgpath := pkgQualifiedPath(p)
		isLocal := true
		for i := 0; isLocal && i < 3 && i < len(mainpath) && i < len(pkgpath); i++ {
			isLocal = isLocal && mainpath[i] == pkgpath[i]
		}
		if isLocal {
			LocalPkgs[p] = true
		}
	}

	opts.OnVerbose(func() {
		fmt.Println("Main packages:")
		for _, p := range mains {
			fmt.Println(p.Pkg.Path())
		}

		fmt.Println("All packages:")
		for _, p := range pkgs {
			fmt.Println(p.Pkg.Path())
		}

		fmt.Println("Local packages:")
		for p := range LocalPkgs {
			fmt.Println(p)
		}
	})

	return
}

func IsLocal(val ssa.Value) bool {
	if val == nil {
		return false
	}

	switch v := val.(type) {
	case *ssa.Function:
		if v != nil && v.Pkg != nil {
			_, ok := LocalPkgs[v.Pkg]
			return ok
		}
	case *ssa.MakeChan:
		fun := v.Parent()
		if fun != nil && fun.Pkg != nil {
			_, ok := LocalPkgs[fun.Pkg]
			return ok
		}
	default:
		if opts.Task().IsGoroTopology() {
			return false
		}
		return IsLocal(v.Parent())
	}
	return false
}
