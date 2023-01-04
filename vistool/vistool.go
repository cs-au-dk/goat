package vistool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"sync"

	"github.com/benbjohnson/immutable"
	ai "github.com/cs-au-dk/goat/analysis/absint"
	"github.com/cs-au-dk/goat/analysis/defs"
	L "github.com/cs-au-dk/goat/analysis/lattice"
	T "github.com/cs-au-dk/goat/analysis/transition"
	u "github.com/cs-au-dk/goat/analysis/upfront"
	"github.com/cs-au-dk/goat/utils"
	"github.com/cs-au-dk/goat/utils/hmap"
	"golang.org/x/tools/go/ssa"
)

type twoWayMapper[T any] struct {
	fw *hmap.Map[T, int]
	bw []T
}

func (tw *twoWayMapper[T]) Id(t T) int {
	v, found := tw.fw.GetOk(t)
	if !found {
		v = tw.fw.Len()
		tw.fw.Set(t, v)
		tw.bw = append(tw.bw, t)
	}
	return v
}

func (tw *twoWayMapper[T]) FromId(id int) (T, bool) {
	if id < 0 || id >= len(tw.bw) {
		var t T
		return t, false
	}
	return tw.bw[id], true
}

func MapperFor[T any](hasher immutable.Hasher[T]) *twoWayMapper[T] {
	return &twoWayMapper[T]{
		fw: hmap.NewMap[int](hasher),
	}
}

var slhsh immutable.Hasher[defs.Superloc] = utils.HashableHasher[defs.Superloc]()

type achasher struct{}

func (achasher) Hash(c *ai.AbsConfiguration) uint32   { return slhsh.Hash(c.Superloc) }
func (achasher) Equal(a, b *ai.AbsConfiguration) bool { return a == b }

type intraprocData struct {
	graph map[defs.CtrLoc][]defs.CtrLoc
	ianal map[defs.CtrLoc]L.AnalysisState
}

func Start(
	C ai.AnalysisCtxt,
	SG ai.SuperlocGraph,
	A L.Analysis,
	blocks ai.Blocks,
) {
	// Give each superlocation a unique ID
	confMapper := MapperFor[*ai.AbsConfiguration](achasher{})

	mux := http.NewServeMux()
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	mux.Handle("/", http.FileServer(http.Dir(path.Join(pwd, "vistool/frontend"))))
	mux.HandleFunc("/graph", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		slId := func(conf *ai.AbsConfiguration) string {
			return fmt.Sprintf("conf-%d", confMapper.Id(conf))
		}
		gId := func(conf *ai.AbsConfiguration, g defs.Goro) string {
			return fmt.Sprintf("%s-%d", slId(conf), g.Hash())
		}

		data := []any{}
		SG.ForEach(func(conf *ai.AbsConfiguration) {
			if conf.IsPanicked() {
				return
			}

			sid := slId(conf)
			sl := conf.Superloc
			slBlocks := blocks[sl]

			sl.ForEach(func(g defs.Goro, cl defs.CtrLoc) {
				lab := cl.Node().Function().String() + "\n" + cl.String()
				_, blocks := slBlocks[g]
				data = append(data, map[string]any{
					"group": "nodes",
					"data": map[string]any{
						"id":     gId(conf, g),
						"parent": sid,
						"str":    lab,
						"blocks": blocks,
					},
					"pannable": true,
				})
			})

			addEdge := func(a, b string, label any) {
				data = append(data, map[string]any{
					"group": "edges",
					"data": map[string]any{
						"id":     fmt.Sprintf("%s-%s", a, b),
						"source": a,
						"target": b,
						"str":    label,
					},
				})
			}

			panics := false
			for _, succ := range conf.Successors {
				conf1 := succ.Configuration()

				if conf1.IsPanicked() {
					panics = true
					continue
				}

				simpleEdge := func(g defs.Goro) {
					addEdge(gId(conf, g), gId(conf1, g), nil)
				}

				// Construct edges between progressed threads based
				// on the type of transition.
				switch tr := succ.Transition().(type) {
				case T.TransitionSingle:
					simpleEdge(tr.Progressed())
					/*
						data = append(data, map[string]any{
							"group": "edges",
							"data": map[string]any{
								"id":     fmt.Sprintf("%s-%s", sid, slId(conf1)),
								"source": sid,
								"target": slId(conf1),
							},
						})
					*/
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
					from1 := gId(conf, tr.Progressed1)
					to1 := gId(conf1, tr.Progressed2)
					from2 := gId(conf, tr.Progressed1)
					to2 := gId(conf1, tr.Progressed2)
					var label string
					// TODO: ...
					s, _ := tr.Channel.GetSite()
					if name, ok := u.ChannelNames[s.Pos()]; ok {
						label = name
					} else {
						label = s.Name()
					}
					// */
					//}
					addEdge(from1, to1, label)
					addEdge(from2, to2, label)
				}
			}

			lab := ""
			if panics {
				lab = "Panics"
			}

			data = append(data, map[string]any{
				"group": "nodes",
				"data": map[string]any{
					"id":           sid,
					"str":          lab,
					"panics":       panics,
					"synchronizes": conf.IsCommunicating(C, A.GetUnsafe(sl)),
					"blocks":       slBlocks != nil,
				},
			})
		})

		json.NewEncoder(w).Encode(data)
		//io.WriteString(w, SG.String())
	})

	var analysisLock sync.Mutex
	analysisCache := map[*ai.AbsConfiguration]map[defs.Goro]intraprocData{}
	mux.HandleFunc("/intraproc", func(w http.ResponseWriter, req *http.Request) {
		if !analysisLock.TryLock() {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, "Server is currently busy")
			return
		}
		defer analysisLock.Unlock()

		err := req.ParseForm()
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, fmt.Sprint(err))
			return
		}

		var slId int
		var gHash uint32
		_, err = fmt.Sscanf(req.FormValue("id"), "conf-%d-%d", &slId, &gHash)
		if err != nil {
			log.Println(err)
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, fmt.Sprint(err))
			return
		}

		log.Println("Request", slId, gHash)
		conf, ok := confMapper.FromId(slId)
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, "Bad superlocation id")
			return
		}

		g, cl, ok := conf.Find(func(g defs.Goro, _ defs.CtrLoc) bool {
			return g.Hash() == gHash
		})
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, "Bad hash")
			return
		}

		log.Println(g, cl)
		idata, found := analysisCache[conf][g]
		if !found {
			initState := A.GetUnsafe(conf.Superloc)
			ianal, graph, _ := conf.IntraprocessualFixpoint(C, g, initState)

			if _, ok := graph[cl]; !ok {
				graph[cl] = nil
			}

			idata = intraprocData{graph, ianal}
			if analysisCache[conf] == nil {
				analysisCache[conf] = map[defs.Goro]intraprocData{}
			}
			analysisCache[conf][g] = idata
		}

		ianal, graph := idata.ianal, idata.graph
		data := []any{}

		addedFuns := map[*ssa.Function]bool{}
		addFun := func(fun *ssa.Function) string {
			id := fmt.Sprintf("fun-%p", fun)
			if !addedFuns[fun] {
				addedFuns[fun] = true

				funId := utils.SSAFunString(fun)
				data = append(data, map[string]any{
					"group": "nodes",
					"data": map[string]any{
						"id":  id,
						"str": funId,
					},
				})
			}
			return id
		}

		addedNodes := map[defs.CtrLoc]bool{}
		addNode := func(cl defs.CtrLoc) string {
			if !addedNodes[cl] {
				addedNodes[cl] = true
				label := cl.String()
				if !cl.Panicked() {
					label += "\n" + ai.StringifyNodeArguments(g, ianal[cl].Memory(), cl.Node())
				}

				data = append(data, map[string]any{
					"group": "nodes",
					"data": map[string]any{
						"id":       strconv.Itoa(int(cl.Hash())),
						"parent":   addFun(cl.Node().Function()),
						"str":      label,
						"deferred": cl.Node().IsDeferred(),
					},
				})

				/*
					if cl.Node().IsDeferred() {
						node.Attrs["fillcolor"] = "#a0ecfa"
					}
				*/
			}
			return strconv.FormatUint(uint64(cl.Hash()), 10)
		}

		for cl, eds := range graph {
			h := addNode(cl)
			for _, ncl := range eds {
				if !cl.Panicked() && ncl.Panicked() {
					// Drop panic-edges for moderately sized graphs.
					if len(graph) > 100 {
						continue
					}
				}

				h2 := addNode(ncl)
				data = append(data, map[string]any{
					"group": "edges",
					"data": map[string]any{
						"id":     fmt.Sprintf("%s-%s", h, h2),
						"source": h,
						"target": h2,
					},
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")

		json.NewEncoder(w).Encode(data)
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("OK"))
		go func() {
			if err := server.Shutdown(context.Background()); err != nil {
				log.Fatal(err)
			}
		}()
	})

	log.Printf("Listening on http://localhost%s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
