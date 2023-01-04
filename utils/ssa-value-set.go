package utils

import (
	"fmt"
	"sort"
	"strings"

	"github.com/benbjohnson/immutable"
	"golang.org/x/tools/go/ssa"
)

type (
	// SSAValueSet is an immutable set of unique SSA values.
	SSAValueSet struct {
		*immutable.Map[ssa.Value, struct{}]
	}

	// ssaValueSetHasher is a hasher for SSA value sets.
	ssaValueSetHasher struct{}
)

// Size returns the number of elements in the SSA value set.
func (s SSAValueSet) Size() int {
	return s.Map.Len()
}

// MakeSSASet creates a set of SSA registers from the given values.
func MakeSSASet(vs ...ssa.Value) SSAValueSet {
	mp := immutable.NewMap[ssa.Value, struct{}](PointerHasher[ssa.Value]{})
	for _, v := range vs {
		mp = mp.Set(v, struct{}{})
	}

	return SSAValueSet{mp}
}

// Add v to s:
//
//	s ∪ {v}
func (s SSAValueSet) Add(v ssa.Value) SSAValueSet {
	return SSAValueSet{s.Map.Set(v, struct{}{})}
}

// Join computes the union of two SSA value sets:
//
//	s1 ∪ s2
func (s1 SSAValueSet) Join(s2 SSAValueSet) SSAValueSet {
	if s1 == s2 {
		return s1
	} else if s2.Size() < s1.Size() {
		s1, s2 = s2, s1
	}

	for iter := s1.Iterator(); !iter.Done(); {
		v, _, _ := iter.Next()
		if !s2.Contains(v) {
			s2.Map = s2.Map.Set(v, struct{}{})
		}
	}

	return s2
}

// Contains checks whether the SSA value set contains v:
//
//	v ∈ s
func (s SSAValueSet) Contains(v ssa.Value) bool {
	_, ok := s.Get(v)
	return ok
}

// Meet computes the intersection of two SSA value sets:
//
//	s1 ∩ s2
func (s1 SSAValueSet) Meet(s2 SSAValueSet) SSAValueSet {
	if s1 == s2 {
		return s1
	}

	vs := make([]ssa.Value, 0, s1.Size())

	s1.ForEach(func(v ssa.Value) {
		if s2.Contains(v) {
			vs = append(vs, v)
		}
	})

	return MakeSSASet(vs...)
}

// ForEach executes the provided procedure for each element in the SSA value set.
func (s SSAValueSet) ForEach(do func(ssa.Value)) {
	for iter := s.Iterator(); !iter.Done(); {
		next, _, _ := iter.Next()
		do(next)
	}
}

// Entries aggregates all elements in the SSA value set in a slice.
func (s SSAValueSet) Entries() []ssa.Value {
	vs := make([]ssa.Value, 0, s.Size())

	s.ForEach(func(v ssa.Value) {
		vs = append(vs, v)
	})
	return vs
}

// Empty checks whether an SSA value set is empty:
//
//	s = ∅
func (s SSAValueSet) Empty() bool {
	return s.Map == nil || s.Map.Len() == 0
}

func (s SSAValueSet) String() string {
	vs := s.Entries()

	// Ensure consistent ordering
	sortingKey := func(v ssa.Value) string {
		res := v.Name() + v.String()
		if f := v.Parent(); f != nil {
			res += f.Prog.Fset.Position(v.Pos()).String()
		}
		return res
	}
	sort.Slice(vs, func(i, j int) bool {
		return sortingKey(vs[i]) < sortingKey(vs[j])
	})

	strs := make([]string, s.Size())

	for i, v := range vs {
		str := v.Name() + " = " + v.String()
		if f := v.Parent(); f != nil {
			str += fmt.Sprintf(" at %v (%v)", f.Prog.Fset.Position(v.Pos()), f)
		}
		strs[i] = str
	}

	return "{ " + strings.Join(strs, "\n") + " }"
}

// Hash computes a hash for the SSA value set, by combining the hashes of individual SSA values,
// in canonical order.
func (ssaValueSetHasher) Hash(s SSAValueSet) uint32 {
	vs := make([]ssa.Value, 0, s.Size())
	s.ForEach(func(v ssa.Value) {
		vs = append(vs, v)
	})
	// Ensure consistent ordering (NOTE: This only works for singleton P-Sets)
	sortingKey := func(v ssa.Value) string {
		if f := v.Parent(); f != nil {
			prog := f.Prog
			return fmt.Sprintf("%s%s%s", v.Name(), v.String(), prog.Fset.Position(v.Pos()))
		}
		return fmt.Sprintf("%s%s%p", v.Name(), v.String(), v)
	}

	sort.Slice(vs, func(i, j int) bool {
		return sortingKey(vs[i]) < sortingKey(vs[j])
	})

	hashes := make([]uint32, 0, s.Size())
	for _, v := range vs {
		hashes = append(hashes, PointerHasher[ssa.Value]{}.Hash(v))
	}

	return HashCombine(hashes...)
}

// Equal checks for equality between two SSA value sets.
func (ssaValueSetHasher) Equal(a, b SSAValueSet) bool {
	if a == b {
		return true
	} else if a.Size() != b.Size() {
		return false
	}

	for it := a.Map.Iterator(); !it.Done(); {
		if k, _, _ := it.Next(); !b.Contains(k) {
			return false
		}
	}
	return true
}

// SSAValueSetHasher is a singleton hasher for SSA value sets.
var SSAValueSetHasher immutable.Hasher[SSAValueSet] = ssaValueSetHasher{}
