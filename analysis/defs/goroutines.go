package defs

import (
	"strconv"

	"github.com/cs-au-dk/goat/utils"

	"github.com/benbjohnson/immutable"
)

//go:generate go run generate-hasher.go goro Goroutine Goro

type Goro interface {
	Hash() uint32
	Equal(Goro) bool
	// Like equal but disregards indexes
	WeakEqual(Goro) bool

	String() string
	// Goroutines use control locations as part of their identification.
	CtrLoc() CtrLoc
	Index() int
	Parent() Goro
	Root() Goro
	// Spawns a goroutine off the current goroutine at the provided control location
	Spawn(CtrLoc) Goro
	// Spawns an indexed goroutine off the current goroutine at the provided control location.
	SpawnIndexed(CtrLoc, int) Goro
	// Sets the index
	SetIndex(int) Goro
	IsRoot() bool

	IsChildOf(Goro) bool
	IsParentOf(Goro) bool

	// Returns true if the control location spawn point chain has a cycle.
	IsCircular() bool
	// Get the parent goroutine with the longest chain of non-repeating control
	// location spawning points.
	GetRadix() Goro
	// Returns the length of the go-string.
	Length() int
}

type goro struct {
	cl     CtrLoc
	parent Goro
	index  int
}

func (factory) Goro(cl CtrLoc, parent Goro) Goro {
	return goro{cl, parent, 0}
}

func (factory) RootGoro(cl CtrLoc) Goro {
	return goro{cl, nil, 0}
}

func (factory) IndexedGoro(cl CtrLoc, parent Goro, index int) Goro {
	return goro{cl, parent, index}
}

func NewGoroutineMap() *immutable.Map {
	return immutable.NewMap(hashergoro{})
}

func (g goro) Hash() (res uint32) {
	if g.parent == nil {
		res = g.cl.Hash()
	} else {
		uinthash := immutable.NewHasher(0)
		res = utils.HashCombine(
			g.parent.Hash(),
			uinthash.Hash(g.index),
			g.cl.Hash())
	}

	// Might need changing
	for res == 0 {
		res = utils.HashCombine(res, res)
	}

	return res
}

func (g goro) CtrLoc() CtrLoc {
	return g.cl
}

func (g goro) Parent() Goro {
	return g.parent
}

func (g goro) Index() int {
	return g.index
}

func (g goro) String() (str string) {
	if g.parent != nil {
		str += g.parent.String() + " ↝ "
	}
	str += colorize.Go(g.cl.String())
	if g.index != 0 {
		str += "(" +
			colorize.Index(strconv.Itoa(g.index)) + ")"
	}
	return
}

func (g goro) Spawn(cl CtrLoc) Goro {
	return goro{cl, g, 0}
}

func (g goro) SpawnIndexed(cl CtrLoc, i int) Goro {
	return goro{cl, g, i}
}

func (g goro) SetIndex(i int) Goro {
	return goro{g.cl, g.parent, i}
}

func (g goro) IsRoot() bool {
	return g.parent == nil
}

func (g1 goro) Equal(g2 Goro) bool {
	if g1.parent == nil {
		return g2.Parent() == nil &&
			g1.cl.Equal(g2.CtrLoc()) &&
			g1.index == g2.Index()
	}
	return g1.cl.Equal(g2.CtrLoc()) &&
		g1.index == g2.Index() &&
		g1.parent.Equal(g2.Parent())
}

func (g1 goro) WeakEqual(g2 Goro) bool {
	if g1.parent == nil {
		return g2.Parent() == nil &&
			g1.cl.Equal(g2.CtrLoc())
	}
	return g1.cl.Equal(g2.CtrLoc()) &&
		g1.parent.WeakEqual(g2.Parent())
}

func (g1 goro) IsChildOf(g2 Goro) bool {
	if g1.parent == nil {
		return false
	}
	return g1.parent.Equal(g2) || g1.parent.IsChildOf(g2)
}

func (g1 goro) IsParentOf(g2 Goro) bool {
	return g2.IsChildOf(g1)
}

func (g1 goro) IsCircular() bool {
	visited := map[CtrLoc]struct{}{
		g1.cl: {},
	}

	for g := g1.parent; g != nil; g = g.Parent() {
		cl := g.CtrLoc()
		if _, found := visited[cl]; found {
			return true
		}
		visited[cl] = struct{}{}
	}

	return false
}

func (g goro) Length() int {
	if g.parent == nil {
		return 1
	}
	return 1 + g.parent.Length()
}

func (g goro) GetRadix() Goro {
	if g.IsCircular() {
		return g.parent.GetRadix()
	}
	return g
}

func (g goro) Root() Goro {
	if g.IsRoot() {
		return g
	}
	return g.parent.Root()
}

type topGoro struct{}

func (factory) TopGoro() Goro {
	return topGoro{}
}

func (topGoro) CtrLoc() CtrLoc {
	panic("Top goroutine has no control location")
}

func (topGoro) Equal(g Goro) bool {
	_, ok := g.(topGoro)
	return ok
}

func (topGoro) GetRadix() Goro {
	panic("Top goroutine has no radix")
}

// TODO: Might need changing
func (topGoro) Hash() uint32 {
	return 0
}

func (topGoro) Index() int {
	panic("Top goroutine has no index")
}

func (topGoro) WeakEqual(g Goro) bool {
	_, ok := g.(topGoro)
	return ok
}

func (topGoro) Length() int {
	panic("Length call on top-goroutine: Top goroutines are not go-strings")
}

func (topGoro) String() string {
	return colorize.Go("[ T-Go ]")
}

func (topGoro) Parent() Goro {
	panic("Top goroutine has no parent")
}
func (topGoro) Root() Goro {
	panic("Top goroutine has no root")
}

func (topGoro) Spawn(CtrLoc) Goro {
	panic("Attempted spawning off the top goroutine")
}

func (topGoro) SpawnIndexed(CtrLoc, int) Goro {
	panic("Attempted indexed spawning off the top goroutine")
}

func (topGoro) SetIndex(int) Goro {
	panic("Attempted setting the index of the top goroutine")
}

func (topGoro) IsRoot() bool {
	panic("Top goroutine cannot be checked as root")
}

func (topGoro) IsChildOf(Goro) bool {
	panic("Top goroutine cannot be a child")
}

func (topGoro) IsParentOf(Goro) bool {
	panic("Top goroutine cannot be a parent")
}

func (topGoro) IsCircular() bool {
	panic("Circularity check on top goroutine")
}
