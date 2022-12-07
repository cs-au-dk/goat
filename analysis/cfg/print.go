package cfg

import (
	"fmt"
	"go/token"
	"log"
)

func printNode(n Node, visited *map[Node]bool) {
	if val, ok := (*visited)[n]; !ok || !val {
		(*visited)[n] = true
		str := n.String()
		postcall := "Post call: "
		successors := "Successors: "
		spawns := "Spawns: "

		if n.DeferLink() != nil {
			if !n.IsDeferred() {
				str += " ---> " + n.DeferLink().String()
			} else {
				str += " <--- " + n.DeferLink().String()
			}
		}

		if n.CallRelation() != nil {
			switch rel := (n.CallRelation()).(type) {
			case *CallNodeRelation:
				postcall += rel.post.String()
			}
		}

		for succ := range n.Successors() {
			successors += " " + succ.String() + ";"
		}

		for spawn := range n.Spawns() {
			spawns += " " + spawn.String() + ";"
		}

		fmt.Println(str)
		if successors != "Successors: " {
			fmt.Println(successors)
		}
		if postcall != "Post call: " {
			fmt.Println(postcall)
		}
		if spawns != "Spawns: " {
			fmt.Println(spawns)
		}
		fmt.Println()

		for succ := range n.Successors() {
			printNode(succ, visited)
		}

		for spawn := range n.Spawns() {
			printNode(spawn, visited)
		}
	}
}

func PrintCfg(G Cfg) {
	var visited *map[Node]bool = new(map[Node]bool)
	*visited = make(map[Node]bool)

	for entry := range G.entries {
		printNode(entry, visited)
	}
}

func PrintCfgFromNode(n Node) {
	var visited *map[Node]bool = new(map[Node]bool)
	*visited = make(map[Node]bool)

	printNode(n, visited)
}

// Print the position of the nearest node with a valid position
func PrintNodePosition(n Node, fs *token.FileSet) bool {
	if n.Pos().IsValid() {
		log.Println("Original construct found at:",
			fs.Position(n.Pos()))
		return true
	}
	for pred := range n.Predecessors() {
		if PrintNodePosition(pred, fs) {
			return true
		}
	}
	for succ := range n.Successors() {
		if PrintNodePosition(succ, fs) {
			return true
		}
	}
	return false
}
