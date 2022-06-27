package graph

import "fmt"

// Source: https://www.cs.rice.edu/~keith/EMBED/dom.pdf

func (G Graph[T]) DominatorTree(root T) func(...T) T {
	postorderTime := G.mapFactory()
	pred := G.mapFactory()

	// Compute DFS post-order ordering
	time := 0
	order := []T{}

	var dfs func(T)
	dfs = func(node T) {
		if _, seen := postorderTime.Get(node); seen {
			return
		}

		postorderTime.Set(node, -1)

		for _, e := range G.Edges(node) {
			var preds []T
			if predsItf, found := pred.Get(e); found {
				preds = predsItf.([]T)
			}

			pred.Set(e, append(preds, node))

			dfs(e)
		}

		postorderTime.Set(node, time)
		order = append(order, node)
		time++
	}

	dfs(root)

	// Initialize doms to "Undefined"
	doms := make([]int, time)
	for i := 0; i < time; i++ {
		doms[i] = -1
	}
	doms[time-1] = time - 1

	intersect := func(a, b int) int {
		for a != b {
			if a < b {
				a = doms[a]
			} else {
				b = doms[b]
			}
		}
		return a
	}

	for {
		changed := false

		// Process nodes in reverse post-order (except for root)
		for i := time - 2; i >= 0; i-- {
			node := order[i]

			new_idom := -1
			predsItf, _ := pred.Get(node)

			for _, predecessor := range predsItf.([]T) {
				jItf, _ := postorderTime.Get(predecessor)
				j := jItf.(int)

				if doms[j] != -1 {
					if new_idom == -1 {
						new_idom = j
					} else {
						new_idom = intersect(j, new_idom)
					}
				}
			}

			if new_idom != doms[i] {
				doms[i] = new_idom
				changed = true
			}
		}

		if !changed {
			break
		}
	}

	return func(nodes ...T) T {
		if len(nodes) == 0 {
			panic("Empty list of nodes for dominator computation")
		}

		dom := -1
		for _, node := range nodes {
			iItf, found := postorderTime.Get(node)
			if !found {
				panic(fmt.Errorf("%v was not reachable when computing the dominator tree", node))
			}

			i := iItf.(int)
			if dom == -1 {
				dom = i
			} else {
				dom = intersect(i, dom)
			}
		}

		return order[dom]
	}
}
