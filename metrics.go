package main

import (
	"fmt"
	"os/exec"

	ai "github.com/cs-au-dk/goat/analysis/absint"
	tu "github.com/cs-au-dk/goat/testutil"
	"golang.org/x/tools/go/ssa"
)

func gatherMetrics(loadRes tu.LoadResult, results map[*ssa.Function]*ai.Metrics) {
	if !opts.Metrics() || len(results) == 0 {
		return
	}
	prog := loadRes.Prog
	coveredConcOp := make(map[ssa.Instruction]struct{})
	coveredChans := make(map[ssa.Instruction]struct{})
	coveredGos := make(map[ssa.Instruction]struct{})

	msg := "================ Results =====================\n\n"

	for f, r := range results {
		msg += "Function: " + f.String() + "\n"
		msg += "Outcome: " + r.Outcome + "\n"

		if r.Outcome == ai.OUTCOME_SKIP {
			msg += "Function finished\n\n"
			continue
		}
		if r.Outcome == ai.OUTCOME_PANIC {
			msg += r.Error() + "\nFunction finished\n\n"
			continue
		}

		msg += "Time: " + r.Performance() + "\n\n"

		files := make(map[string]struct{})

		if len(r.Functions()) > 0 {
			msg += "Expanded functions: " + fmt.Sprintf("%d", len(r.Functions())) + " {\n"
			for f, times := range r.Functions() {
				fn := prog.Fset.Position(f.Pos()).Filename
				if _, ok := files[fn]; !ok {
					files[prog.Fset.Position(f.Pos()).Filename] = struct{}{}
				}

				msg += "  " + f.String() + " -- " + fmt.Sprintf("%d", times) + "\n"
			}
			msg += "}\n"
		}

		if len(r.Blocks()) > 0 {
			msg += "Blocks:"
			msg += r.Blocks().String()
			msg += "\n"
		}

		fs := make([]string, 0, len(files))
		for f := range files {
			fs = append(fs, f)
		}
		if len(fs) > 0 {
			cloc := exec.Command("cloc", fs...)
			out, err := cloc.Output()
			if err == nil {
				msg += string(out) + "\n"
			}
		}

		for i := range r.ConcurrencyOps() {
			coveredConcOp[i] = struct{}{}
		}
		for i := range r.Gos() {
			coveredGos[i] = struct{}{}
		}
		for i := range r.Chans() {
			coveredChans[i] = struct{}{}
		}
		msg += "Function finished\n\n"
	}

	allConcOps := loadRes.Cfg.GetAllConcurrencyOps()
	msg += "Concurrency operations covered: " + fmt.Sprint(len(coveredConcOp)) + "/" + fmt.Sprint(len(allConcOps)) + " {\n"
	if len(allConcOps) > 0 {
		notCovered := make(map[ssa.Instruction]struct{})
		for op := range allConcOps {
			if _, ok := coveredConcOp[op]; !ok {
				notCovered[op] = struct{}{}
			}
		}
		if len(notCovered) > 0 {
			msg += "Not covered: {\n"
			for op := range notCovered {
				msg += "  " + op.String() + ":" + prog.Fset.Position(op.Pos()).String() + "\n"
			}
			msg += "}\n"
		}
	}

	allChans := loadRes.Cfg.GetAllChans()
	msg += "Channel sites covered: " + fmt.Sprint(len(coveredChans)) + "/" + fmt.Sprint(len(allChans)) + "\n"
	if len(allChans) > 0 {
		notCovered := make(map[ssa.Instruction]struct{})
		for ch := range allChans {
			if _, ok := coveredChans[ch]; !ok {
				notCovered[ch] = struct{}{}
			}
		}
		if len(notCovered) > 0 {
			msg += "Not covered: {\n"
			for ch := range notCovered {
				msg += "  " + ch.String() + ":" + prog.Fset.Position(ch.Pos()).String() + "\n"
			}
			msg += "}\n"
		}
	}

	allGos := loadRes.Cfg.GetAllGos()
	msg += "Goroutine sites covered: " + fmt.Sprint(len(coveredGos)) + "/" + fmt.Sprint(len(allGos)) + "\n"
	if len(allGos) > 0 {
		notCovered := make(map[ssa.Instruction]struct{})
		for g := range allGos {
			if _, ok := coveredGos[g]; !ok {
				notCovered[g] = struct{}{}
			}
		}
		if len(notCovered) > 0 {
			msg += "Not covered: {\n"
			for g := range notCovered {
				msg += "  " + g.String() + ":" + prog.Fset.Position(g.Pos()).String() + "\n"
			}
			msg += "}\n"
		}
	}
	msg += "================ Results ====================="
	fmt.Println(msg)
}
