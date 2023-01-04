package absint

import (
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/cs-au-dk/goat/analysis/defs"
	T "github.com/cs-au-dk/goat/analysis/transition"
	"github.com/cs-au-dk/goat/utils/dot"
)

const condensePanickedConfigurations = true

// Construct a Dot graph given a starting coarse configuration.
func (s0 *AbsConfiguration) OldVisualize() {
	G := &dot.DotGraph{
		Options: map[string]string{
			"minlen":  fmt.Sprint(opts.Minlen()),
			"nodesep": fmt.Sprint(opts.Nodesep()),
			"rankdir": "TB",
			"label":   "Futures for thread " + s0.Main().String(),
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
		itids := make([]int, 0, s.Size())
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
			loc, _ := s.Get(g)
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
			ID: s0.Main().String() + ":start",
			Attrs: dot.DotAttrs{
				"fillcolor": "#99d7f7",
				"label":     s0.GetUnsafe(s0.Main()).Node().String(),
			},
		}},
		Attrs: dot.DotAttrs{
			"label":   "Target operation on thread " + s0.Main().String(),
			"bgcolor": "#AAF7FF",
		},
	}

	G.Clusters = []*dot.DotCluster{targetCluster}
	addConfigurationCluster(s0, s0.Main())

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
					synced := s0.Main()
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
					addConfigurationCluster(conf1, s0.Main())
					queue = append(queue, conf1)
				}

				var progressed defs.Goro
				switch tr := succ.Transition().(type) {
				case T.In:
					progressed = tr.Progressed()
				case T.Send:
					progressed = tr.Progressed()
				case T.Receive:
					progressed = tr.Progressed()
				case T.Close:
					progressed = tr.Progressed()
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
