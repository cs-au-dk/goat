package absint

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"Goat/analysis/cfg"
	"Goat/analysis/defs"
	T "Goat/analysis/transition"
	"Goat/analysis/upfront"
	"Goat/utils/dot"
)

const condensePanickedConfigurations = true

// Construct a Dot graph given a starting coarse configuration.
func (s0 *AbsConfiguration) OldVisualize() {
	G := &dot.DotGraph{
		Options: map[string]string{
			"minlen":  fmt.Sprint(opts.Minlen()),
			"nodesep": fmt.Sprint(opts.Nodesep()),
			"rankdir": "TB",
			"label":   "Futures for thread " + s0.Target.String(),
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

	queue := []*AbsConfiguration{s0}
	// Give configuration unique identifiers by incrementing a counter
	configCounter := 0

	addConfigurationCluster := func(s *AbsConfiguration, synced defs.Goro) {
		// Choose a different bgcolor if the target thread has progressed
		bgcolor := "#FFD581"
		if s.Threads().GetUnsafe(s.Target) != s0.Threads().GetUnsafe(s.Target) {
			bgcolor = "#00e544"
		}

		idStr := strconv.Itoa(configCounter)
		configCounter++
		configurationToCluster[s] = &ConfigurationCluster{
			Threads: make(map[uint32]*dot.DotNode),
			Cluster: &dot.DotCluster{
				ID: idStr,
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
				ID: idStr + ":" + g.String(),
				Attrs: dot.DotAttrs{
					"label": g.String() + " @ " + loc.String(),
					"style": "filled",
				},
			}

			// If the thread synchronized with the target, mark it with a different color.
			if synced.Equal(g) {
				threadNode.Attrs["fillcolor"] = "#99d7f7"
			} else {
				threadNode.Attrs["fillcolor"] = "#FFE691"
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
	}

	targetCluster := &dot.DotCluster{
		ID: "Target operation",
		Nodes: []*dot.DotNode{{
			ID: s0.Target.String() + ":start",
			Attrs: dot.DotAttrs{
				"fillcolor": "#99d7f7",
				"label":     s0.Threads().GetUnsafe(s0.Target).Node().String(),
			},
		}},
		Attrs: dot.DotAttrs{
			"label":   "Target operation on thread " + s0.Target.String(),
			"bgcolor": "#AAF7FF",
		},
	}

	G.Clusters = []*dot.DotCluster{targetCluster}
	addConfigurationCluster(s0, s0.Target)

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

			// Construct edges between progressed threads based
			// on the type of transition.
			if tr, isSync := succ.Transition().(T.Sync); isSync {
				if configurationToCluster[conf1] == nil {
					synced := s0.Target
					switch {
					case tr.Progressed1.Equal(s0.Target):
						synced = tr.Progressed2
					case tr.Progressed2.Equal(s0.Target):
						synced = tr.Progressed1
					}
					addConfigurationCluster(conf1, synced)
					queue = append(queue, conf1)
				}
				from1 := configurationToCluster[conf].Threads[tr.Progressed1.Hash()]
				to1 := configurationToCluster[conf1].Threads[tr.Progressed1.Hash()]
				from2 := configurationToCluster[conf].Threads[tr.Progressed2.Hash()]
				to2 := configurationToCluster[conf1].Threads[tr.Progressed2.Hash()]
				var label string
				/* TODO: ...
				if name, ok := upfront.ChannelNames[tr.Channel.Pos()]; ok {
					label = name
				} else {
				*/
				s, _ := tr.Channel.GetSite()
				label = s.Name()
				//}
				attrs := dot.DotAttrs{"label": label}
				addEdge(from1, to1, attrs)
				addEdge(from2, to2, attrs)
			} else {
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1, s0.Target)
					queue = append(queue, conf1)
				}

				var progressed defs.Goro
				switch tr := succ.Transition().(type) {
				case T.In:
					progressed = tr.Progressed
				case T.Send:
					progressed = tr.Progressed
				case T.Receive:
					progressed = tr.Progressed
				case T.Close:
					progressed = tr.Progressed
				default:
					log.Fatalf("Unknown transition type: %T %v", tr, tr)
				}

				from := configurationToCluster[conf].Threads[progressed.Hash()]
				to := configurationToCluster[conf1].Threads[progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			}
		}
	}

	G.ShowDot()
}

// Construct a Dot graph given a starting coarse configuration.
func (s0 *AbsConfiguration) Visualize() {
	htoa := func(hash uint32) string {
		return strconv.FormatUint(uint64(hash), 10)
	}
	gCounter := 0
	gToColor := make(map[defs.Goro]string)
	colors := []string{
		"#E8FBE1",
		"#ECE3FC",
		"#FCEBF6",
		"#FAF8DF",
		"#DDF2FD",
		"#DEE0E4",
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

	queue := []*AbsConfiguration{s0}

	addConfigurationCluster := func(s *AbsConfiguration) {
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
			if _, ok := gToColor[g]; !ok {
				gToColor[g] = colors[gCounter%len(colors)]
				gCounter++
			}
			loc, _ := s.Threads().Get(g)
			threadNode := &dot.DotNode{
				ID: htoa(s.Hash()) + ":" + htoa(g.Hash()),
				Attrs: dot.DotAttrs{
					"label":     loc.Node().Function().String() + "\n" + loc.String(),
					"style":     "filled",
					"fillcolor": gToColor[g],
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

			// Construct edges between progressed threads based
			// on the type of transition.
			switch tr := succ.Transition().(type) {
			case T.In:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			case T.Send:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			case T.Receive:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			case T.Close:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			case T.Lock:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			case T.Unlock:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			case T.RLock:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			case T.RUnlock:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from, to, dot.DotAttrs{})
			case T.Broadcast:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from := configurationToCluster[conf].Threads[tr.Broadcaster.Hash()]
				to := configurationToCluster[conf1].Threads[tr.Broadcaster.Hash()]
				addEdge(from, to, dot.DotAttrs{})
				for broadcastee := range tr.Broadcastees {
					from := configurationToCluster[conf].Threads[broadcastee.Hash()]
					to := configurationToCluster[conf1].Threads[broadcastee.Hash()]
					addEdge(from, to, dot.DotAttrs{})
				}
			case T.Wake:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from1 := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to1 := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from1, to1, dot.DotAttrs{})
			case T.Wait:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from1 := configurationToCluster[conf].Threads[tr.Progressed.Hash()]
				to1 := configurationToCluster[conf1].Threads[tr.Progressed.Hash()]
				addEdge(from1, to1, dot.DotAttrs{})
			case T.Signal:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
				from1 := configurationToCluster[conf].Threads[tr.Progressed1.Hash()]
				to1 := configurationToCluster[conf1].Threads[tr.Progressed1.Hash()]
				addEdge(from1, to1, dot.DotAttrs{})
				if !tr.Missed() {
					from2 := configurationToCluster[conf].Threads[tr.Progressed2.Hash()]
					to2 := configurationToCluster[conf1].Threads[tr.Progressed2.Hash()]
					addEdge(from2, to2, dot.DotAttrs{})
				}
			case T.Sync:
				if configurationToCluster[conf1] == nil {
					addConfigurationCluster(conf1)
					queue = append(queue, conf1)
				}
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
			}
		}
	}

	G.ShowDot()
}
