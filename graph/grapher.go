package graph

import (
	"bytes"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/dot"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/pointer"
	"golang.org/x/tools/go/ssa"
)

const (
	chanClusterKey = "$channels"
	sharedPackage  = "$shared_pkg"
)

var opts = utils.Opts()

var (
	constructLabel = struct {
		Val func(*ssa.Program, ssa.Value) string
		Ins func(*ssa.Program, ssa.Instruction) string
	}{
		Val: func(prog *ssa.Program, val ssa.Value) string {
			pos := prog.Fset.Position(val.Pos())
			return fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line)
		},
		Ins: func(prog *ssa.Program, ins ssa.Instruction) string {
			pos := prog.Fset.Position(ins.Pos())
			return fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line)
		},
	}
)

func isSynthetic(edge *callgraph.Edge) bool {
	return edge.Caller.Func.Pkg == nil || edge.Callee.Func.Synthetic != ""
}

func packageName(pkg *ssa.Package) string {
	if pkg == nil {
		return "nil"
	}
	return pkg.Pkg.Path()
}

func functionId(function *ssa.Function) string {
	if pos := function.Pos(); pos != 0 {
		return fmt.Sprintf("%d", pos)
	}
	return fmt.Sprintf("%s_0", packageName(function.Pkg))
}

func makePkgCluster(fun *ssa.Function, clusterMap map[string]*dot.DotCluster) *dot.DotCluster {
	//defer timeTrack(time.Now(), fmt.Sprintf("makePkgCluster %+v", fun))

	var key, label string
	// var pkg *build.Package
	if fun != nil && fun.Pkg != nil {
		key = fun.Pkg.Pkg.Path()
		label = fun.Pkg.Pkg.Name()
	} else {
		key = sharedPackage
		label = "Shared package"
	}
	if _, ok := clusterMap[key]; !ok {
		color := "#cff3ff"

		// Check if package exists in GOROOT to determine if it's part of the standard library
		if key != sharedPackage {
			path := filepath.Join(runtime.GOROOT(), "src", key)
			if fi, err := os.Stat(path); err == nil && fi.IsDir() {
				color = "#E0FFE1"
			}
		}

		clusterMap[key] = &dot.DotCluster{
			ID:       key,
			Clusters: make(map[string]*dot.DotCluster),
			Attrs: dot.DotAttrs{
				"penwidth":  "0.8",
				"fontsize":  "24",
				"label":     fmt.Sprintf("%s", label),
				"style":     "filled",
				"fillcolor": color,
				"fontname":  "Tahoma bold",
				"tooltip":   fmt.Sprintf("package: %s", key),
				"rank":      "sink",
			},
		}
	}
	return clusterMap[key]
}

func makeChanCluster(parent *dot.DotCluster) *dot.DotCluster {
	key := parent.ID
	clusters := parent.Clusters
	if _, ok := clusters[chanClusterKey]; !ok {
		clusters[chanClusterKey] = &dot.DotCluster{
			ID: parent.ID + ":$channels",
			Attrs: dot.DotAttrs{
				"penwidth":  "0.8",
				"fontsize":  "16",
				"label":     "Channel allocation sites",
				"style":     "filled",
				"fillcolor": "pink",
				"fontname":  "Tahoma bold",
				"tooltip":   fmt.Sprintf("Channel allocations in: %s", key),
				"rank":      "sink",
			},
		}
	}
	return clusters[chanClusterKey]
}

func boringGoroutine(goro *upfront.Goro) bool {
	// Change to 'len(goro.SpawnedGoroutines)+len(goro.CrossPackageCalls) == 0'
	//  to factor in CrossPackageCalls when deciding if a package is interesting.
	if opts.JustGoros() || opts.FullCg() {
		return false
	}
	var noKids bool

	if opts.PackageSplit() {
		noKids = (len(goro.SpawnedGoroutines) + len(goro.CrossPackageCalls)) == 0
	} else {
		noKids = len(goro.SpawnedGoroutines) == 0
	}
	noChanOps := true
	for range goro.ChannelOperations {
		noChanOps = false
		break
	}
	return noKids && noChanOps
}

func BuildGraph(prog *ssa.Program, result *pointer.Result, goros []*upfront.Goro) string {
	// Compute position string relative to entire program
	getPosString := func(tok token.Pos, fallback string) string {
		if pos := prog.Fset.Position(tok); pos.IsValid() {
			return pos.String()
		}

		return fallback
	}

	getFunPosString := func(fun *ssa.Function) string {
		return getPosString(fun.Pos(), fmt.Sprintf("%s_%s", packageName(fun.Pkg), fun.Name()))
	}

	var (
		nodes    []*dot.DotNode
		edges    []*dot.DotEdge
		clusters []*dot.DotCluster
	)

	nodeMap := make(map[string]*dot.DotNode)
	edgeMap := make(map[string]*dot.DotEdge)
	clusterMap := make(map[string]*dot.DotCluster)

	for _, goro := range goros {
		// Goroutines with no channel usage or successors are boring
		if boringGoroutine(goro) {
			continue
		}

		cluster := makePkgCluster(goro.Entry, clusterMap)
		gokey := strings.Replace(getFunPosString(goro.Entry), string(filepath.Separator), "/", -1)

		n := &dot.DotNode{
			ID:    gokey,
			Attrs: make(dot.DotAttrs),
		}

		n.Attrs["fillcolor"] = "lightblue"
		n.Attrs["label"] = goro.Entry.Name()

		nodeMap[gokey] = n
		cluster.Nodes = append(cluster.Nodes, n)

		// Process edges to channels
		for ch, props := range goro.ChannelOperations {
			parentCluster := makePkgCluster(ch.Parent(), clusterMap)
			chkey := strings.Replace(getPosString(ch.Pos(), "-"), string(filepath.Separator), "/", -1)
			chcluster := makeChanCluster(parentCluster)
			if _, ok := nodeMap[chkey]; !ok {
				label := constructLabel.Val(prog, ch)
				if name, ok := upfront.ChannelNames[ch.Pos()]; ok {
					label = name + " - " + label
				}
				n = &dot.DotNode{
					ID:    chkey,
					Attrs: make(dot.DotAttrs),
				}
				n.Attrs["fillcolor"] = "magenta"
				n.Attrs["label"] = label
				n.Attrs["tooltip"] = strings.Replace(getPosString(goro.Entry.Pos(), "-"), string(filepath.Separator), "/", -1)
				nodeMap[chkey] = n
				chcluster.Nodes = append(chcluster.Nodes, n)
			}

			ekey := fmt.Sprintf("%s => %s", gokey, chkey)
			if _, ok := edgeMap[ekey]; !ok {
				edgeMap[ekey] = &dot.DotEdge{
					From:  nodeMap[gokey],
					To:    nodeMap[chkey],
					Attrs: make(dot.DotAttrs),
				}
			}
			edge := edgeMap[ekey]
			tooltip := []string{}
			edge.Attrs["arrowhead"] = ""
			if props.Make != nil {
				edge.Attrs["arrowhead"] = edge.Attrs["arrowhead"] + "dot"
				edge.Attrs["color"] = "blue"
				edge.Attrs["label"] = props.BufferLabel()
				label := constructLabel.Ins(prog, props.Make)
				tooltip = append(tooltip, "Make operation at: "+label+"\n")
			}
			if len(props.Send) > 0 {
				edge.Attrs["arrowhead"] = edge.Attrs["arrowhead"] + "vee"
				tooltip = append(tooltip, "Send operations at:")
				for _, i := range props.Send {
					label := constructLabel.Ins(prog, i)
					tooltip = append(tooltip, label)
				}
			}
			if len(props.Receive) > 0 {
				edge.Attrs["arrowhead"] = edge.Attrs["arrowhead"] + "crow"
				tooltip = append(tooltip, "\nReceive operations at:")
				for _, i := range props.Receive {
					label := constructLabel.Ins(prog, i)
					tooltip = append(tooltip, label)
				}
			}
			if opts.Extended() && len(props.Close) > 0 {
				edge.Attrs["arrowhead"] = edge.Attrs["arrowhead"] + "box"
				tooltip = append(tooltip, "\nClose operations at:")
				for _, i := range props.Close {
					label := constructLabel.Ins(prog, i)
					tooltip = append(tooltip, label)
				}
			}
			edge.Attrs["tooltip"] = strings.Join(tooltip, "\n")
		}
	}

	// Process edges between goroutines
	addGoroEdge := func(from, to *upfront.Goro, isGo bool) {
		goroKey := strings.Replace(getFunPosString(from.Entry), string(filepath.Separator), "/", -1)
		// Child goroutine was not assigned a node, therefore it was not interesting
		childKey := strings.Replace(getFunPosString(to.Entry), string(filepath.Separator), "/", -1)
		if _, ok := nodeMap[childKey]; !ok {
			return
		}
		ekey := fmt.Sprintf("%s --> %s", goroKey, childKey)
		if _, ok := edgeMap[ekey]; !ok {
			edge := &dot.DotEdge{
				From:  nodeMap[goroKey],
				To:    nodeMap[childKey],
				Attrs: make(dot.DotAttrs),
			}
			edge.Attrs["arrowhead"] = "onormal"
			if !isGo {
				edge.Attrs["style"] = "dashed"
			}

			edgeMap[ekey] = edge
		}
	}

	for _, goro := range goros {
		if boringGoroutine(goro) {
			continue
		}

		for _, child := range goro.SpawnedGoroutines {
			addGoroEdge(goro, child, true)
		}

		// Uncomment to add CrossPackageCall edges
		for _, child := range goro.CrossPackageCalls {
			addGoroEdge(goro, child, false)
		}
	}

	// get edges form edgeMap
	for _, e := range edgeMap {
		e.From.Attrs["tooltip"] = fmt.Sprintf(
			"%s\n%s",
			e.From.Attrs["tooltip"],
			e.Attrs["tooltip"],
		)
		edges = append(edges, e)
	}

	for _, c := range clusterMap {
		clusters = append(clusters, c)
	}

	dotG := &dot.DotGraph{
		Title:    "Goroutine topology",
		Clusters: clusters,
		Nodes:    nodes,
		Edges:    edges,
		Options: map[string]string{
			"minlen":  fmt.Sprint(opts.Minlen()),
			"nodesep": fmt.Sprint(opts.Nodesep()),
		},
	}

	fmt.Printf("Clusters: %d\nNodes: %d\nEdges: %d\n",
		len(clusters), len(nodeMap), len(edgeMap))

	var buf bytes.Buffer
	if err := dotG.WriteDot(&buf); err != nil {
		fmt.Println(err)
		return ""
	}

	out, err := dot.DotToImage("", opts.OutputFormat(), buf.Bytes())
	if err != nil {
		fmt.Println(err)
		return ""
	}
	return out
}
