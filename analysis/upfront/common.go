package upfront

import (
	"github.com/cs-au-dk/goat/pkgutil"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

// Short-hands for commonly used variables and operations
var (
	opts         = utils.Opts()
	task         = opts.Task()
	verbosePrint = utils.VerbosePrint

	// isLocal checks that an SSA value is in a local package.
	isLocal = func(v ssa.Value) bool {
		if !opts.LocalPackages() {
			return true
		}
		return pkgutil.IsLocal(v)
	}
)
