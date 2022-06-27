package upfront

import (
	"Goat/pkgutil"
	"Goat/utils"

	"golang.org/x/tools/go/ssa"
)

var (
	opts         = utils.Opts()
	task         = opts.Task()
	verbosePrint = utils.VerbosePrint
	isLocal      = func(v ssa.Value) bool {
		if !opts.LocalPackages() {
			return true
		}
		return pkgutil.IsLocal(v)
	}
)
