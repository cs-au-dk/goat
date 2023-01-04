package main

import (
	"fmt"
	"log"

	"github.com/cs-au-dk/goat/analysis/cfg"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	"golang.org/x/tools/go/ssa"
)

// pipeline is a wrapper around the analysis pipeline.
type pipeline struct {
	prog  *ssa.Program
	mains []*ssa.Package
}

// standardPTAnalysisQueries is a standard set of queries for types to include in the Andersen points-to analysis.
var standardPTAnalysisQueries = u.IncludeType{
	Chan:      true,
	Function:  true,
	Interface: true,
}

// preanalysisPipeline executes parts of the pre-analysis,
// by constructing the enhanced CFG and the points-to analysis.
func (p pipeline) preanalysisPipeline(includes u.IncludeType) (*u.PointerResult, *cfg.Cfg) {
	fmt.Println()
	log.Println("Performing points-to analysis...")
	ptaResult := u.Andersen(p.prog, p.mains, includes)
	log.Println("Points-to analysis done")
	fmt.Println()

	log.Println("Extending CFG...")
	progCfg := cfg.GetCFG(p.prog, p.mains, &ptaResult.Result)
	log.Println("CFG extensions done")
	fmt.Println()

	opts.OnVerbose(func() {
		for val, ptr := range ptaResult.Queries {
			fmt.Printf("Points to information for \"%s\" at %d (%s):\n",
				val, val.Pos(), p.prog.Fset.Position(val.Pos()))
			for _, label := range ptr.PointsTo().Labels() {
				fmt.Printf("%s : %d (%s), ", label, (*label).Pos(), p.prog.Fset.Position((*label).Pos()))
			}
			fmt.Print("\n\n")
		}
	})

	return ptaResult, progCfg
}

// fullPreanalysisPipeline executes the pre-analysis by performing
// a full pre-analysis.
func (p pipeline) fullPreanalysisPipeline(includes u.IncludeType) (
	*u.PointerResult,
	*cfg.Cfg,
	u.GoTopology,
) {
	ptaResult, progCfg := p.preanalysisPipeline(includes)

	log.Println("Constructing Goroutine topology...")
	goros := u.CollectGoros(&ptaResult.Result)
	log.Println("Goroutine topology done")

	opts.OnVerbose(func() {
		fmt.Println("Found the following goroutines:")
		for _, goro := range goros {
			fmt.Println(goro.String())
			fmt.Println()
		}
		fmt.Println()
	})

	return ptaResult, progCfg, goros
}
