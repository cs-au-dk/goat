package defs

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/cs-au-dk/goat/utils"

	i "github.com/cs-au-dk/goat/utils/indenter"

	"github.com/benbjohnson/immutable"
)

type (
	// pathCondition is currently unused.s
	pathCondition struct{}

	// Superloc is an configuration mapping abstract threads to control locations,
	// denoting the next instruction they will execute.
	Superloc struct {
		threads *immutable.Map[Goro, CtrLoc]
		pc      pathCondition
	}
)

// Superloc creates a superlocation value from a map from goroutines to threads.
func (factory) Superloc(threads map[Goro]CtrLoc) Superloc {
	mp := immutable.NewMapBuilder[Goro, CtrLoc](utils.HashableHasher[Goro]())
	for tid, loc := range threads {
		mp.Set(tid, loc)
	}
	return Superloc{
		mp.Map(),
		struct{}{},
	}
}

func (factory) EmptySuperloc() Superloc {
	return Create().Superloc(nil)
}

// Method for retrieving the main goroutine (identified as the root goroutine).
func (s Superloc) Main() Goro {
	for iter := s.threads.Iterator(); !iter.Done(); {
		g, _, _ := iter.Next()
		if g.IsRoot() {
			return g
		}
	}
	panic(fmt.Sprintf("No main goroutine for superlocation %s", s))
}

// Size returns the number of active goroutines in a superlocation.
func (s Superloc) Size() int {
	return s.threads.Len()
}

// GetUnsafe returns the control location for a given goroutine.
//
// Will throw a fatal exception if the goroutine is not found.
func (s Superloc) GetUnsafe(g Goro) CtrLoc {
	cl, found := s.threads.Get(g)
	if !found {
		log.Fatal("Queried for inexistent thread", g, "in superlocation", s)
	}
	return cl
}

// Get retrieves the control location of a given goroutine,
func (s Superloc) Get(tid Goro) (CtrLoc, bool) {
	return s.threads.Get(tid)
}

// Hash computes a 32-bit hash for a given superlocation.
func (s Superloc) Hash() uint32 {
	hashes := []uint32{}
	iter := s.threads.Iterator()

	for !iter.Done() {
		g, cl, _ := iter.Next()
		hashes = append(hashes, utils.HashCombine(g.Hash(), cl.Hash()))
	}

	sort.Slice(hashes, func(i, j int) bool {
		return hashes[i] < hashes[j]
	})

	return utils.HashCombine(hashes...)
}

func (s Superloc) String() string {
	hashes := []func() string{}
	iter := s.threads.Iterator()

	for !iter.Done() {
		g, cl, _ := iter.Next()
		hashes = append(hashes, func() string {
			var clstr string
			if cl.Panicked() {
				clstr = colorize.Panic(cl.String())
			} else {
				clstr = colorize.Superloc(cl.String())
			}
			return g.String() + " ⇒ " + clstr
		})
	}

	return i.Indenter().Start("⟨").NestThunkedPresep("| ", hashes...).End("⟩")
}

// StringWithPos constructs a string representation of the superlocation,
// where the control location is represented as a position in the source.
func (s Superloc) StringWithPos() string {
	hashes := []string{}
	iter := s.threads.Iterator()

	for !iter.Done() {
		g, cl, _ := iter.Next()
		hashes = append(hashes, func() string {
			var clstr string
			if cl.Panicked() {
				clstr = colorize.Panic(cl.String())
			} else {
				clstr = colorize.Superloc(cl.String())
			}
			if pos := cl.PosString(); pos != "" {
				clstr += "\n" + pos
			}
			return g.String() + " ⇒ " + clstr
		}())
	}

	return "⟨ " + strings.Join(hashes, "\n| ")
}

// ForEach executes the given procedure for every goroutine-control location
// pair in the superlocation.
func (s Superloc) ForEach(do func(Goro, CtrLoc)) {
	iter := s.threads.Iterator()
	for !iter.Done() {
		g, cl, _ := iter.Next()
		do(g, cl)
	}
}

// Find checks whether a goroutine-control location pair in the superlocation
// satisfies the given predicate, and returns them, and true if found, or false
// if not found.
func (s Superloc) Find(find func(Goro, CtrLoc) bool) (Goro, CtrLoc, bool) {
	iter := s.threads.Iterator()
	for !iter.Done() {
		g, cl, _ := iter.Next()
		if find(g, cl) {
			return g, cl, true
		}
	}

	return goro{}, CtrLoc{}, false
}

// FindAll returns a binding of goroutines to control locations for every
// goroutine-control location pair that satisfies the given predicate.
func (s Superloc) FindAll(find func(Goro, CtrLoc) bool) map[Goro]CtrLoc {
	res := make(map[Goro]CtrLoc)
	s.ForEach(func(g Goro, cl CtrLoc) {
		if find(g, cl) {
			res[g] = cl
		}
	})

	return res
}

// Derive constructs a new superlocation that inherits all the bindings
// of the given superlocation, and then overrides them with the binding given
// by the threads map.
func (s Superloc) Derive(threads map[Goro]CtrLoc) Superloc {
	for g, cl := range threads {
		s.threads = s.threads.Set(g, cl)
	}
	return s
}

// DeriveThread constructs a new goroutine where the given goroutine
// has been updated to the given control location.
func (s Superloc) DeriveThread(g Goro, cl CtrLoc) Superloc {
	s.threads = s.threads.Set(g, cl)
	return s
}

// Equal checks for equality between superlocations.
func (s1 Superloc) Equal(s2 Superloc) bool {
	if s1.Size() != s2.Size() {
		return false
	}

	_, _, res := s1.Find(func(g Goro, cl1 CtrLoc) bool {
		cl2, found := s2.Get(g)
		return !(found && cl1.Equal(cl2))
	})
	return !res
}

// Root returns the main goroutine of the superlocation.
func (s Superloc) Root() Goro {
	g, _, _ := s.Find(func(g Goro, cl CtrLoc) bool {
		return true
	})

	return g.Root()
}

// NextIndex returns the next available index for the given goroutine.
// If the goroutine is not bound in the superlocation, next index is 0.
func (s Superloc) NextIndex(g1 Goro) int {
	gs := s.FindAll(func(g2 Goro, cl CtrLoc) bool {
		return g1.WeakEqual(g2)
	})
	max := -1
	for g := range gs {
		i := g.Index()
		if i > max {
			max = i
		}
	}
	return max + 1
}
