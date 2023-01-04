package defs

import (
	"strconv"

	"github.com/cs-au-dk/goat/utils"

	"github.com/benbjohnson/immutable"
)

// Goro represents abstract threads.
type Goro interface {
	String() string

	// Hash computes the 32-bit hash of a given goroutine.
	Hash() uint32

	// Equal compares strict equality between goroutines.
	Equal(Goro) bool
	// WeakEqual compares for equality between goroutines, disregarding indexes
	WeakEqual(Goro) bool

	// CtrLoc retrieves the control location used to spawn a goroutine. Control locations are used as part of goroutine identification.
	CtrLoc() CtrLoc
	// Index retrieves the index of a goroutine, denoting how many identical goroutines were created beforehand.
	Index() int
	// Parent retrieves the parent goroutine that spawned the current goroutine.
	Parent() Goro
	// Root retrieves the main goroutine, from which all other goroutines are transitively spawned.
	Root() Goro
	// Spawn constructs a goroutine spawned from the current goroutine at the provided control location
	Spawn(CtrLoc) Goro
	// SpawnIndexed constructs an indexed goroutine spawned from the current goroutine at the provided control location.
	SpawnIndexed(CtrLoc, int) Goro
	// SetIndex constructs a goroutine from the given goroutine, with the given index.
	SetIndex(int) Goro
	// IsRoot checks whether the given goroutine is the main goroutine.
	IsRoot() bool

	// IsChildOf checks whether the given goroutine is the child of the current goroutine.
	IsChildOf(Goro) bool
	// IsChildOf checks whether the given goroutine is the parent of the current goroutine.
	IsParentOf(Goro) bool

	// IsCircular is  true if the control location spawn point chain has a cycle.
	IsCircular() bool
	// GetRadix computes the parent goroutine with the longest chain of non-repeating control
	// location spawning points.
	GetRadix() Goro

	// Returns the length of the go-string.
	Length() int
}

type (
	// goro is a known abstract thread. It is used for threads that belong to the given fragment.
	goro struct {
		cl     CtrLoc
		parent Goro
		index  int
	}

	// topGoro is an unknown abstract thread. It is used to encode heap locations
	// outside the fragment.
	topGoro struct{}
)

// Goro creates an abstract goroutine from the given control location, and parent goroutine.
func (factory) Goro(cl CtrLoc, parent Goro) Goro {
	return goro{cl, parent, 0}
}

// RootGoro creates an abstract goroutine denoting the main thread.
func (factory) RootGoro(cl CtrLoc) Goro {
	return goro{cl, nil, 0}
}

// IndexedGoro creates an abstract goroutine with the given index.
func (factory) IndexedGoro(cl CtrLoc, parent Goro, index int) Goro {
	return goro{cl, parent, index}
}

// Hash computes the 32-bit hash of a given goroutine.
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

// CtrLoc retrieves the control location of a goroutine.
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
