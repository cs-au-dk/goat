package absint

import (
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"

	"github.com/cs-au-dk/goat/analysis/cfg"
	"github.com/cs-au-dk/goat/analysis/defs"
	T "github.com/cs-au-dk/goat/analysis/transition"
	"github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/utils/dot"
)

// Construct a dot graphed based on the superlocation graph
func (SG SuperlocGraph) Visualize(blocks Blocks) {
	htoa := func(hash uint32) string {
		return strconv.FormatUint(uint64(hash), 10)
	}

	G := &dot.DotGraph{
		Options: map[string]string{
			"minlen":  fmt.Sprint(opts.Minlen()),
			"nodesep": fmt.Sprint(opts.Nodesep()),
			"rankdir": "TB",
			// "label":   "Futures for thread " + s0.Target.String(),
			// Necessary to ensure that ordering within clusters is kept.
			// See https://stackoverflow.com/questions/33959969/order-cluster-nodes-in-graphviz
			"remincross": "false",
		},
	}
	fmt.Println()

	// A configuration cluster maps every thread to a CFG Dot node for easy access.
	type ConfigurationCluster struct {
		Threads map[uint32]*dot.DotNode
		Cluster *dot.DotCluster
	}
	configurationToCluster := make(map[*AbsConfiguration]*ConfigurationCluster)

	addEdge := func(from, to *dot.DotNode, attrs dot.DotAttrs) {
		G.Edges = append(G.Edges, &dot.DotEdge{
			From:  from,
			To:    to,
			Attrs: attrs,
		})
	}

	s0 := SG.Entry()
	var queue []*AbsConfiguration

	addConfigurationCluster := func(s *AbsConfiguration) {
		if configurationToCluster[s] != nil {
			return
		}

		// Choose a different bgcolor if the target thread has progressed
		bgcolor := "#FFD581"
		// if s.Threads().GetUnsafe(s.Target) != s0.Threads().GetUnsafe(s.Target) {
		// 	bgcolor = "#00e544"
		// }
		if len(s.Successors) == 0 {
			_, _, blocking := s.superloc.Find(func(g defs.Goro, cl defs.CtrLoc) bool {
				_, ok := cl.Node().(*cfg.TerminateGoro)
				return !ok
			})
			if blocking {
				bgcolor = "#CC0000"
			}
		}

		configurationToCluster[s] = &ConfigurationCluster{
			Threads: make(map[uint32]*dot.DotNode),
			Cluster: &dot.DotCluster{
				ID: htoa(s.Hash()),
				Attrs: dot.DotAttrs{
					"bgcolor": bgcolor,
				},
			},
		}

		// Ensure consistent ordering in cluster by sorting by thread ID
		gs := make(map[uint32]defs.Goro)
		itids := make([]int, 0, s.Threads().Size())
		s.ForEach(func(g defs.Goro, _ defs.CtrLoc) {
			gs[g.Hash()] = g
			itids = append(itids, (int)(g.Hash()))
		})

		sort.Ints(itids)
		nodeIds := []string{}
		tids := make([]uint32, 0, len(itids))
		for _, tid := range itids {
			tids = append(tids, (uint32)(tid))
		}

		var prevNode *dot.DotNode = nil
		for _, tid := range tids {
			// if g == s0.Target {
			// 	continue
			// }
			g := gs[tid]
			loc, _ := s.Threads().Get(g)
			threadNode := &dot.DotNode{
				ID: htoa(s.Hash()) + ":" + htoa(g.Hash()),
				Attrs: dot.DotAttrs{
					"label": loc.Node().Function().String() + "\n" + loc.String(),
					"style": "filled",
				},
			}

			if loc.Panicked() {
				threadNode.Attrs["fillcolor"] = "#dd493b"
			}

			configurationToCluster[s].Threads[tid] = threadNode
			configurationToCluster[s].Cluster.Nodes = append(configurationToCluster[s].Cluster.Nodes, threadNode)

			nodeIds = append(nodeIds, fmt.Sprintf("%q", threadNode.ID))

			if prevNode != nil {
				// Add invisible edges that enforce an ordering within the cluster
				addEdge(prevNode, threadNode, dot.DotAttrs{
					"style":  "invis",
					"minlen": "0",
				})
			}

			prevNode = threadNode
		}

		// We cant use rankdir="LR" within a cluster so we have to manually assign
		// all the nodes to the same rank.
		rankStr := fmt.Sprintf("{rank=same; %s}", strings.Join(nodeIds, " "))
		configurationToCluster[s].Cluster.Prefix = rankStr

		G.Clusters = append(G.Clusters, configurationToCluster[s].Cluster)
		queue = append(queue, s)
	}

	// targetCluster := &dot.DotCluster{
	// 	ID: "Target operation",
	// 	Nodes: []*dot.DotNode{{
	// 		ID: s0.Target.String() + ":start",
	// 		Attrs: dot.DotAttrs{
	// 			"fillcolor": "#99d7f7",
	// 			"label":     s0.Threads().GetUnsafe(s0.Target).Node().String(),
	// 		},
	// 	}},
	// 	Attrs: dot.DotAttrs{
	// 		"label":   "Target operation on thread " + s0.Target.String(),
	// 		"bgcolor": "#AAF7FF",
	// 	},
	// }

	G.Clusters = []*dot.DotCluster{}
	addConfigurationCluster(s0)

	for len(queue) > 0 {
		conf := queue[0]
		queue = queue[1:]

		for _, succ := range conf.Successors {
			conf1 := succ.Configuration()

			if condensePanickedConfigurations && conf1.IsPanicked() {
				cluster := configurationToCluster[conf]
				cluster.Cluster.Attrs["label"] = "Panics"
				continue
			}

			addConfigurationCluster(conf1)

			simpleEdge := func(g defs.Goro) {
				from := configurationToCluster[conf].Threads[g.Hash()]
				to := configurationToCluster[conf1].Threads[g.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			}

			// Construct edges between progressed threads based
			// on the type of transition.
			switch tr := succ.Transition().(type) {
			case T.TransitionSingle:
				simpleEdge(tr.Progressed())
			case T.Broadcast:
				simpleEdge(tr.Broadcaster)
				for broadcastee := range tr.Broadcastees {
					simpleEdge(broadcastee)
				}
			case T.Signal:
				simpleEdge(tr.Progressed1)
				if !tr.Missed() {
					simpleEdge(tr.Progressed2)
				}
			case T.Sync:
				from1 := configurationToCluster[conf].Threads[tr.Progressed1.Hash()]
				to1 := configurationToCluster[conf1].Threads[tr.Progressed1.Hash()]
				from2 := configurationToCluster[conf].Threads[tr.Progressed2.Hash()]
				to2 := configurationToCluster[conf1].Threads[tr.Progressed2.Hash()]
				var label string
				// TODO: ...
				s, _ := tr.Channel.GetSite()
				if name, ok := upfront.ChannelNames[s.Pos()]; ok {
					label = name
				} else {
					label = s.Name()
				}
				// */
				//}
				attrs := dot.DotAttrs{"label": label}
				addEdge(from1, to1, attrs)
				addEdge(from2, to2, attrs)

			default:
				panic(fmt.Errorf("Unknown transition type: %T %v", tr, tr))
			}
		}
	}

	// Sloppy code for marking blocked threads according to the deduplicated
	// blocking information available in `blocks`.
	sgGraph := SG.ToGraph()
	for sl, gs := range blocks {
		colors := map[defs.Goro]string{}
		for g := range gs {
			colors[g] = fmt.Sprintf("#cc%02x%02x", rand.Intn(256), rand.Intn(256))
		}

		sgGraph.BFS(SG.Get(sl), func(s *AbsConfiguration) bool {
			c := configurationToCluster[s]
			for g := range gs {
				c.Threads[g.Hash()].Attrs["fillcolor"] = colors[g]
			}
			return false
		})
	}

	G.ShowDot()
}
