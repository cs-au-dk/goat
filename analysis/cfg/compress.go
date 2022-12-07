package cfg

func removeSuccessor(from Node, to Node) {
	from.removeSuccessor(to)
	to.removePredecessor(from)
}

func (cfg *Cfg) removeNode(node Node) {
	for pred := range node.Predecessors() {
		removeSuccessor(pred, node)
	}
	for succ := range node.Successors() {
		removeSuccessor(node, succ)
	}
	if _, isEntry := cfg.entries[node]; isEntry {
		delete(cfg.entries, node)
		for succ := range node.Successors() {
			cfg.entries[succ] = struct{}{}
		}
	}
	fun := node.Function()
	delete(cfg.funs[fun].nodes, node)

	// If the node being removed has a defer link,
	// and that node's defer link is still the removed node,
	// then set it to nil instead.
	if dfr := node.DeferLink(); dfr != nil {
		if dfr.DeferLink() == node {
			dfr.addDeferLink(nil)
		}
	}

	switch node := node.(type) {
	case *SSANode:
		ins := node.Instruction()
		delete(cfg.nodeToInsn, node)
		delete(cfg.insnToNode, ins)
	case AnySynthetic:
		id := node.Id()
		delete(cfg.synthetics, id)
	}
}

func compress(cfg *Cfg) {
	deferBookkeeping := make(map[Node]map[Node]struct{})

	visited := make(map[Node]bool)
	queue := []Node{}

	manageDeferrers := func(node Node, overtaker Node) {
		if node.IsDeferred() {
			if _, found := deferBookkeeping[overtaker]; !found {
				deferBookkeeping[overtaker] = make(map[Node]struct{})
			}
			if dfr := node.DeferLink(); dfr != nil {
				dfr.addDeferLink(overtaker)
				deferBookkeeping[overtaker][dfr] = struct{}{}
			}
			for dfr := range deferBookkeeping[node] {
				dfr.addDeferLink(overtaker)
				deferBookkeeping[overtaker][dfr] = struct{}{}
			}
			delete(deferBookkeeping, node)
		}
	}

	compress := func(node Node) {
		switch {
		case len(node.Successors()) == 0:
			// Compressible nodes without successors should be discarded.
			// Any predecessors are re-added to the queue, so they may
			// be removed if compressible.
			for pred := range node.Predecessors() {
				visited[pred] = false
				queue = append(queue, pred)
			}
			cfg.removeNode(node)
		case len(node.Successors()) == 1:
			// If a compressible node has a single successor, discard it
			// and create an edge from each predecessor to the successor.
			succ := node.Successor()
			for pred := range node.Predecessors() {
				visited[pred] = false
				queue = append(queue, pred)
				SetSuccessor(pred, succ)
			}
			// If any other nodes have this node as a panic continuation,
			// shift the continuation to the successor.
			for pnc := range node.Panickers() {
				setPanicCont(pnc, succ)
			}
			// Shift the defer link(s) to the successor of the removed node.
			manageDeferrers(node, succ)
			visited[succ] = false
			queue = append(queue, succ)
			cfg.removeNode(node)
			return
		case len(node.Predecessors()) == 1:
			// If a compressible node has a single predecessor, discard it
			// and create an edge between from the predecessor to each successor.
			pred := node.Predecessor()

			// We can only shift panic continuations backwards if the precessor
			// is also a deferred node. (This prevents shifting the panic
			// continuation back into normal flow.)
			if node.IsDeferred() && !pred.IsDeferred() {
				return
			}

			visited[pred] = false
			queue = append(queue, pred)
			for succ := range node.Successors() {
				visited[succ] = false
				queue = append(queue, succ)
				SetSuccessor(pred, succ)
			}
			// If any other nodes have this node as a panic continuation,
			// shift the continuation to the predecessor.
			for pnc := range node.Panickers() {
				setPanicCont(pnc, pred)
			}
			// Shift the defer link to the predecessor of the removed node.
			manageDeferrers(node, pred)
			cfg.removeNode(node)
			return
		}

		// Compressible nodes should not have a self-directed edge.
		for succ := range node.Successors() {
			if succ == node {
				removeSuccessor(node, node)
				// Add the node to the queue, since it may now be further compressed.
				visited[node] = false
				queue = append(queue, node)
			}
		}
	}

	for entry := range cfg.entries {
		queue = append(queue, entry)
	}

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		// If settled is false, the node was previously visited,
		// but re-added to the compression queue due to a change
		// in neighbor cardinality.
		if settled, ok := visited[node]; !settled || !ok {
			visited[node] = true

			switch node := node.(type) {
			case *BlockEntry:
				compress(node)
			case *BlockExitDefer:
				compress(node)
			case *SelectDefer:
				compress(node)
			case *BlockExit:
				compress(node)
			case *BlockEntryDefer:
				compress(node)
			}

			// Due to being reachable, continuation and spawn nodes
			// may be added to the queue.
			for succ := range node.Continuations() {
				queue = append(queue, succ)
			}
			for spawn := range node.Spawns() {
				queue = append(queue, spawn)
			}
		}
	}

	// Discard unreachable CFG nodes.
	for insNode := range cfg.nodeToInsn {
		if _, ok := visited[insNode]; !ok {
			cfg.removeNode(insNode)
		}
	}
	for _, synthetic := range cfg.synthetics {
		if _, ok := visited[synthetic]; !ok {
			cfg.removeNode(synthetic)
		}
	}

	exists := func(node Node) bool {
		var ok bool
		switch n := node.(type) {
		case *SSANode:
			_, ok = cfg.nodeToInsn[n]
		case AnySynthetic:
			_, ok = cfg.synthetics[n.Id()]
		default:
			panic("???")
		}
		return ok
	}

	// For inifinite loop blocks, if the panic continuation is missing,
	// set it to the exit node of the parent function.
	// For instruction nodes:
	// TODO: This is not right. It will fail to capture the correct panic
	// TODO: continuation if the block is not the entry block.
	// TODO: It is OK only as long as the panic continuation nodes are not pursued
	// TODO: during abstract interpretation.
	for insNode := range cfg.nodeToInsn {
		// fmt.Println(insNode, "here1")
		// Nodes without a panic continuation do not need rewiring
		if pnc := insNode.PanicCont(); pnc != nil {
			// If the panic continuation was visited, no action
			// must be taken
			if !exists(pnc) {
				// Othwerise, search for the function exit node and wire it
				setPanicCont(insNode, cfg.funs[insNode.Function()].exit)
			}
		}
	}
	// For synthetic nodes:
	for _, synthetic := range cfg.synthetics {
		// Nodes without a panic continuation do not need rewiring
		if pnc := synthetic.PanicCont(); pnc != nil {
			// If the panic continuation was visited, no action
			// must be taken
			if !exists(pnc) {
				// Othwerise, search for the function exit node and wire it
				setPanicCont(synthetic, cfg.funs[synthetic.Function()].exit)
			}
		}
	}
}
