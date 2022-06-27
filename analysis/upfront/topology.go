package upfront

import (
	"fmt"
	"math"
	"path/filepath"

	"golang.org/x/tools/go/ssa"
)

type GoTopology []*Goro

func (goros GoTopology) LogCycles() {
	indexes := make(map[*Goro]int)
	lenGoros := len(goros)

	dist := make([][]int, lenGoros)
	next := make([][]*Goro, lenGoros)

	makeLabel := func(fun *ssa.Function) string {
		pos := fun.Prog.Fset.Position(fun.Pos())
		path := fmt.Sprintf("%s/%s", fun.Package().Pkg.Path(), filepath.Base(pos.Filename))
		return path + ":" + fun.Name()
	}

	for i, goro := range goros {
		indexes[goro] = i
	}

	for i := 0; i < lenGoros; i++ {
		dist[i] = make([]int, lenGoros)
		next[i] = make([]*Goro, lenGoros)
		for j := 0; j < lenGoros; j++ {
			dist[i][j] = math.MaxInt32
			next[i][j] = nil
		}
	}

	for _, goro := range goros {
		hosti := indexes[goro]
		for _, spawned := range goro.SpawnedGoroutines {
			guesti := indexes[spawned]
			dist[hosti][guesti] = 1
			next[hosti][guesti] = spawned
		}
	}

	for k := 0; k < lenGoros; k++ {
		for i := 0; i < lenGoros; i++ {
			for j := 0; j < lenGoros; j++ {
				if dist[i][j] > (dist[i][k] + dist[k][j]) {
					dist[i][j] = dist[i][k] + dist[k][j]
					next[i][j] = next[i][k]
				}
			}
		}
	}

	foundCycle := "none"
	for i, goro := range goros {
		switch dist[i][i] {
		case math.MaxInt32:
		case 1:
			if foundCycle == "none" {
				foundCycle = ""
			}
			foundCycle += fmt.Sprintf("\n:~ 1 @@ %s", makeLabel(goro.Entry))
		default:
			if foundCycle == "none" {
				foundCycle = ""
			}
			foundCycle += fmt.Sprintf("\n:~ %d @@ %s", dist[i][i], makeLabel(goro.Entry))
			succ := next[i][i]
			for goro != succ {
				foundCycle += " -▶ " + makeLabel(succ.Entry)
				succi := indexes[succ]
				succ = next[succi][i]
			}
		}
	}

	fmt.Println(foundCycle)
}

// Calculate cycles in the call graph.
func (goros GoTopology) Cycles() GoCycles {
	indexes := make(map[*Goro]int)
	lenGoros := len(goros)

	dist := make([][]int, lenGoros)
	next := make([][]*Goro, lenGoros)

	for i, goro := range goros {
		indexes[goro] = i
	}

	for i := 0; i < lenGoros; i++ {
		dist[i] = make([]int, lenGoros)
		next[i] = make([]*Goro, lenGoros)
		for j := 0; j < lenGoros; j++ {
			dist[i][j] = math.MaxInt32
			next[i][j] = nil
		}
	}

	for _, goro := range goros {
		hosti := indexes[goro]
		for _, seq := range goro.CrossPackageCalls {
			guesti := indexes[seq]
			dist[hosti][guesti] = 1
			next[hosti][guesti] = seq
		}
		for _, spawned := range goro.SpawnedGoroutines {
			guesti := indexes[spawned]
			dist[hosti][guesti] = 1
			next[hosti][guesti] = spawned
		}
	}

	for k := 0; k < lenGoros; k++ {
		for i := 0; i < lenGoros; i++ {
			for j := 0; j < lenGoros; j++ {
				if dist[i][j] > (dist[i][k] + dist[k][j]) {
					dist[i][j] = dist[i][k] + dist[k][j]
					next[i][j] = next[i][k]
				}
			}
		}
	}

	cycles := map[*Goro][]*Goro{}
	for i, goro := range goros {
		switch dist[i][i] {
		case math.MaxInt32:
		default:
			cycles[goro] = []*Goro{goro}

			succ := next[i][i]

			for goro != succ {
				cycles[goro] = append(cycles[goro], succ)
				succi := indexes[succ]
				succ = next[succi][i]
			}
		}
	}

	return cycles
}

func (goros GoTopology) FunToGoro(f *ssa.Function) *Goro {
	for _, g := range goros {
		if g.Entry == f {
			return g
		}
	}

	return nil
}

func (goros GoTopology) HasCycle(g *Goro) bool {
	return goros.Cycles().HasCycle(g)
}

func (goros GoTopology) FunHasCycle(f *ssa.Function) bool {
	return goros.Cycles().FunHasCycle(f)
}

type GoCycles map[*Goro][]*Goro

func (goros GoCycles) FunToGoro(f *ssa.Function) *Goro {
	for g := range goros {
		if g.Entry == f {
			return g
		}
	}

	return nil
}

func (goro GoCycles) HasCycle(g *Goro) bool {
	_, ok := goro[g]
	return ok
}

func (goros GoCycles) FunHasCycle(f *ssa.Function) bool {
	return goros.HasCycle(goros.FunToGoro(f))
}

type GoCollection interface {
	HasCycle(*Goro) bool

	FunToGoro(*ssa.Function) *Goro
	FunHasCycle(*ssa.Function) bool
}
