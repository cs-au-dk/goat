package lattice

import (
	"fmt"
	"sort"

	"github.com/cs-au-dk/goat/utils"
	i "github.com/cs-au-dk/goat/utils/indenter"
	"github.com/cs-au-dk/goat/utils/tree"

	"github.com/benbjohnson/immutable"
)

// InfSetLattice is a powerset lattice derived from the (possibly) infinite set of elements of the given type.
type InfSetLattice[T any] struct {
	lattice
	hasher immutable.Hasher[T]
}

// Bot returns the empty set belonging to the infinite powerset lattice.
func (m *InfSetLattice[T]) Bot() Element {
	return InfSet[T]{element{m}, tree.NewTree[T, struct{}](m.hasher)}
}

// Top cannot be invoked on the infinite powerset lattice.
func (m *InfSetLattice[T]) Top() Element {
	panic(errUnsupportedOperation)
}

// Eq checks for referencial equality with another powerset lattice.
func (m *InfSetLattice[T]) Eq(o Lattice) bool {
	return m == o
}

func (m *InfSetLattice[T]) String() string {
	var t T
	return fmt.Sprintf("Infinite Powerset[%T]", t)
}

var _ Lattice = &InfSetLattice[int]{}

func MakeInfSetLattice[T any](hasher immutable.Hasher[T]) *InfSetLattice[T] {
	return &InfSetLattice[T]{hasher: hasher}
}

func MakeInfSetLatticeH[T utils.HashableEq[T]]() *InfSetLattice[T] {
	return &InfSetLattice[T]{hasher: utils.HashableHasher[T]()}
}

// InfSet is a member of a given infinite powerset.
type InfSet[T any] struct {
	element
	set tree.Tree[T, struct{}]
}

var infSetMergeFunc = func(a, b struct{}) (struct{}, bool) {
	return a, true
}

func (e1 InfSet[T]) Add(t T) InfSet[T] {
	e1.set = e1.set.InsertOrMerge(t, struct{}{}, infSetMergeFunc)
	return e1
}

func (e1 InfSet[T]) Contains(t T) bool {
	_, found := e1.set.Lookup(t)
	return found
}

func (e1 InfSet[T]) ForEach(f func(T)) {
	e1.set.ForEach(func(t T, _ struct{}) { f(t) })
}

func (e1 InfSet[T]) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 InfSet[T]) eq(e2 Element) bool {
	return e1.set.Equal(e2.(InfSet[T]).set, func(a, b struct{}) bool { return true })
}

func (e1 InfSet[T]) Geq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 InfSet[T]) geq(e2 Element) bool {
	return e2.leq(e1) // OBS
}

func (e1 InfSet[T]) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 InfSet[T]) leq(e2 Element) bool {
	return e1.join(e2).eq(e2)
}

func (e1 InfSet[T]) Join(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 InfSet[T]) join(e2 Element) Element {
	return e1.MonoJoin(e2.(InfSet[T]))
}

func (e1 InfSet[T]) MonoJoin(e2 InfSet[T]) InfSet[T] {
	e1.set = e1.set.Merge(e2.set, infSetMergeFunc)
	return e1
}

func (e1 InfSet[T]) Meet(e2 Element) Element {
	checkLatticeMatch(e1.Lattice(), e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 InfSet[T]) meet(e2 Element) Element {
	return e1.MonoMeet(e2.(InfSet[T]))
}

func (e1 InfSet[T]) MonoMeet(e2 InfSet[T]) InfSet[T] {
	panic(errNotImplemented)
}

func (e1 InfSet[T]) String() string {
	buf := []string{}

	e1.ForEach(func(k T) {
		buf = append(buf, fmt.Sprint(k))
	})

	if len(buf) == 0 {
		return colorize.Element("∅")
	}

	sort.Strings(buf)
	return i.Indenter().Start("{").
		NestStringsSep(",", buf...).
		End("}")
}

var _ Element = InfSet[int]{}
