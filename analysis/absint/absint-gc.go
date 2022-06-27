package absint

import (
	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	L "Goat/analysis/lattice"
	loc "Goat/analysis/location"

	"golang.org/x/tools/go/ssa"
)

// Overapproximates whether it is safe to run `abstractGC` on the memory when
// returning from `from`. It is safe to GC when exiting a function if we cannot
// end up returning into the same function later.
// This is trivially true if the function is not in a cycle in the callgraph
// (but the opposite does not hold - it can be safe to gc when leaving a function
// in a cycle if the function we are leaving to is outside the cycle (TODO)).
// Checking if the function is in a callgraph cycle is overapproximated
// by checking if the function exit node can reach itself sequentially in the
// overapproximated exploded CFG.
// It can be made more precise by considering the actual call graph (instead
// of the exploded CFG, which may contain cycles that do not correspond to
// call graph cycles), or even goroutine-specific call graphs.
func canGC(g defs.Goro, from *cfg.FunctionExit) bool {
	return !cfg.SequentiallySelfReaching(from)
}

// This function garbage collects (some) values in the memory that are no
// longer needed after goroutine `g` exits `fun`.
func abstractGC(g defs.Goro, fun *ssa.Function, mem L.Memory) L.Memory {
	// Collect all values local to the function.
	// NOTE: Do not GC return value
	var valuesToGC []ssa.Value
	for _, param := range fun.Params {
		valuesToGC = append(valuesToGC, param)
	}
	for _, fv := range fun.FreeVars {
		valuesToGC = append(valuesToGC, fv)
	}
	for _, block := range fun.Blocks {
		for _, insn := range block.Instrs {
			if val, ok := insn.(ssa.Value); ok {
				valuesToGC = append(valuesToGC, val)
			}
		}
	}

	// Remove all the collected values from the memory
	for _, val := range valuesToGC {
		mem = mem.Remove(loc.LocationFromSSAValue(g, val))
	}

	// TODO: After this step we could try to look for allocation sites that are
	// unreachable from GC roots (like a real GC). This requires defining what
	// a root is (probably locals (above) and globals) and tracing the heap
	// from those.
	// Return values are also roots - it requires additional reasoning to know
	// when we can GC return values.
	return mem
}
