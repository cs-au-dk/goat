package chreflect

import (
	"go/types"

	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/utils"

	"golang.org/x/tools/go/ssa"
)

// GetReflectedChannels computes the set of channels that flow directly into the reflect.ValueOf function.
// Does not consider channels that flow indirectly, such as through struct fields.
func GetReflectedChannels(prog *ssa.Program, pt *u.PointerResult) utils.SSAValueSet {
	// Create a fresh set of SSA values.
	res := utils.MakeSSASet()

	if pkg := prog.ImportedPackage("reflect"); pkg != nil {
		fun := pkg.Func("ValueOf")

		// The pointer analysis ignores the parameters of functions in the
		// reflect library, so we cannot simply lookup the points-to result
		// for the argument of ValueOf.
		if node, found := pt.CallGraph.Nodes[fun]; found {
			for _, edge := range node.In {
				args := edge.Site.Common().Args
				if len(args) != 1 {
					// Maybe there's some weird interface call thing that I
					// haven't considered...
					continue
				}

				// The parameter has interface type, so we have to dereference first.
				arg := args[0]
				for _, itfLabel := range pt.Queries[arg].PointsTo().Labels() {
					value := itfLabel.Value()
					if value == nil {
						continue
					}

					mkItf, ok := value.(*ssa.MakeInterface)
					if !ok {
						continue
					}

					// We only care about channels (can be extended if we want)
					if _, isChan := mkItf.X.Type().Underlying().(*types.Chan); !isChan {
						continue
					}

					for _, xLabel := range pt.Queries[mkItf.X].PointsTo().Labels() {
						if chnSite := xLabel.Value(); chnSite != nil {
							res = res.Add(chnSite)
						}
					}
				}
			}
		}
	}

	return res
}
