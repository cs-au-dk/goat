package tree

import (
	"fmt"

	i "github.com/cs-au-dk/goat/utils/indenter"

	"github.com/benbjohnson/immutable"
)

// Constructs a new persistent key-value map with the specified hasher.
func NewTree[K, V any](hasher immutable.Hasher[K]) Tree[K, V] {
	return Tree[K, V]{hasher, nil}
}

type Tree[K, V any] struct {
	hasher immutable.Hasher[K]
	root   node[K, V]
}

func (tree Tree[K, V]) Lookup(key K) (V, bool) {
	// Hashing can be expensive, so we hash the key once here and pass it on.
	return lookup(tree.root, tree.hasher.Hash(key), key, tree.hasher)
}

// Inserts the given key-value pair into the map.
// Replaces previous value with the same key if it exists.
func (tree Tree[K, V]) Insert(key K, value V) Tree[K, V] {
	return tree.InsertOrMerge(key, value, nil)
}

// Inserts the given key-value pair into the map. If a previous mapping
// (prevValue) exists for the key, the inserted value will be `f(value, prevValue)`.
func (tree Tree[K, V]) InsertOrMerge(key K, value V, f mergeFunc[V]) Tree[K, V] {
	tree.root, _ = insert(tree.root, tree.hasher.Hash(key), key, value, tree.hasher, f)
	return tree
}

// Remove a mapping for the given key if it exists.
func (tree Tree[K, V]) Remove(key K) Tree[K, V] {
	// TODO: We can check if the key exists before erasing to prevent
	// replacing parts of subtrees unnecessarily (to preserve pointer equality)
	tree.root = remove(tree.root, tree.hasher.Hash(key), key, tree.hasher)
	return tree
}

// Call the given function once for each key-value pair in the map.
func (tree Tree[K, V]) ForEach(f eachFunc[K, V]) {
	if tree.root != nil {
		tree.root.each(f)
	}
}

// Merges two maps. If both maps contain a value for a key, the resulting map
// will map the key to the result of `f` on the two values.
// `f` must be commutative and idempotent!
// This operation is made fast by skipping processing of shared subtrees.
// Merging a tree with itself after r updates should have complexity
// equivalent to `O(r * (keysize + f))`.
func (tree Tree[K, V]) Merge(other Tree[K, V], f mergeFunc[V]) Tree[K, V] {
	tree.root, _ = merge(tree.root, other.root, tree.hasher, f)
	return tree
}

// Returns whether two maps are equal. Values are compared with the provided
// function. This operation also skips processing of shared subtrees.
func (tree Tree[K, V]) Equal(other Tree[K, V], f cmpFunc[V]) bool {
	return equal(tree.root, other.root, tree.hasher, f)
}

// Returns the number of key-value pairs in the map.
// NOTE: Runs in linear time in the size of the map.
func (tree Tree[K, V]) Size() (res int) {
	tree.ForEach(func(_ K, _ V) {
		res++
	})
	return
}

func (tree Tree[K, V]) StringFiltered(pred func(k K, v V) bool) string {
	buf := []func() string{}

	tree.ForEach(func(k K, v V) {
		if pred(k, v) {
			buf = append(buf, func() string {
				return fmt.Sprintf("%v ↦ %v", k, v)
			})
		}
	})

	// sort.Slice(buf, func(i, j int) bool {
	// 	return buf[i]() < buf[j]()
	// })
	return i.Indenter().Start("{").NestThunked(buf...).End("}")
}

func (tree Tree[K, V]) String() string {
	return tree.StringFiltered(func(_ K, _ V) bool { return true })

}

// End of public interface

// The patricia tree implementation is based on:
// http://ittc.ku.edu/~andygill/papers/IntMap98.pdf

type eachFunc[K, V any] func(key K, value V)
type node[K, V any] interface {
	each(eachFunc[K, V])
}

type keyt = uint32

type branch[K, V any] struct {
	prefix keyt // Common prefix of all keys in the left and right subtrees
	// A number with exactly one positive bit. The position of the bit
	// determines where the prefixes of the left and right subtrees diverge.
	branchBit keyt
	left      node[K, V]
	right     node[K, V]
}

func (b *branch[K, V]) each(f eachFunc[K, V]) {
	b.left.each(f)
	b.right.each(f)
}

// Returns whether the key matches the prefix up until the branching bit.
// Intuitively: does the key belong in the branch's subtree?
func (b *branch[K, V]) match(key keyt) bool {
	return (key & (b.branchBit - 1)) == b.prefix
}

type pair[K, V any] struct {
	key   K
	value V
}
type leaf[K, V any] struct {
	// The (shared) hash value of all keys in the leaf.
	key keyt
	// List of values to handle hash collisions.
	// TODO: Since collisions should be rare it might be worth
	// it to have a fast implementation when no collisions occur.
	values []pair[K, V]
}

func (l *leaf[K, V]) copy() *leaf[K, V] {
	return &leaf[K, V]{
		l.key,
		append([]pair[K, V](nil), l.values...),
	}
}

func (l *leaf[K, V]) each(f eachFunc[K, V]) {
	for _, pr := range l.values {
		f(pr.key, pr.value)
	}
}

// Smart branch constructor
func br[K, V any](prefix, branchBit keyt, left, right node[K, V]) node[K, V] {
	if left == nil {
		return right
	} else if right == nil {
		return left
	}

	return &branch[K, V]{prefix, branchBit, left, right}
}

// Recursive lookup on tree.
func lookup[K, V any](tree node[K, V], hash keyt, key K, hasher immutable.Hasher[K]) (ret V, found bool) {
	if tree == nil {
		return
	}

	switch tree := tree.(type) {
	case *leaf[K, V]:
		if tree.key == hash {
			for _, pr := range tree.values {
				if hasher.Equal(key, pr.key) {
					return pr.value, true
				}
			}
		}

		return

	case *branch[K, V]:
		rec := tree.right
		if !tree.match(hash) {
			return
		} else if zeroBit(hash, tree.branchBit) {
			rec = tree.left
		}

		return lookup(rec, hash, key, hasher)

	default:
		panic("???")
	}
}

// Joins two trees t0 and t1 which have prefixes p0 and p1 respectively.
// The prefixes must not be equal!
func join[K, V any](p0, p1 keyt, t0, t1 node[K, V]) node[K, V] {
	bbit := branchingBit(p0, p1)
	prefix := p0 & (bbit - 1)
	if zeroBit(p0, bbit) {
		return &branch[K, V]{prefix, bbit, t0, t1}
	} else {
		return &branch[K, V]{prefix, bbit, t1, t0}
	}
}

// Merges two values. Must be commutative and idempotent.
// The second return value informs the caller whether a == b.
// NOTE: This flag allows us to do some optimizations. Namely we can keep old
// nodes instead of replacing them with "equal" copies when the flag is true.
// However, it complicates the implementation a little bit - I'm not sure it's
// worth it.
type mergeFunc[V any] func(a, b V) (V, bool)

// If `f` is nil the old value is always replaced with the argument value, otherwise
// the old value is replaced with `f(value, prevValue)`.
// If the returned flag is false, the returned node is (reference-)equal to the input node.
func insert[K, V any](tree node[K, V], hash keyt, key K, value V, hasher immutable.Hasher[K], f mergeFunc[V]) (node[K, V], bool) {
	if tree == nil {
		return &leaf[K, V]{key: hash, values: []pair[K, V]{{key, value}}}, true
	}

	var prefix keyt
	switch tree := tree.(type) {
	case *leaf[K, V]:
		if tree.key == hash {
			for i, pr := range tree.values {
				// If key matches previous key, replace value
				if hasher.Equal(key, pr.key) {
					newValue := value
					if f != nil {
						var equal bool
						newValue, equal = f(value, pr.value)

						if equal {
							return tree, false
						}
					}

					lf := tree.copy()
					lf.values[i].value = newValue
					return lf, true
				}
			}

			// Hash collision - append to list of values in leaf
			lf := tree.copy()
			lf.values = append(lf.values, pair[K, V]{key, value})
			return lf, true
		}

		prefix = tree.key

	case *branch[K, V]:
		if tree.match(hash) {
			l, r := tree.left, tree.right
			var changed bool
			if zeroBit(hash, tree.branchBit) {
				l, changed = insert(l, hash, key, value, hasher, f)
			} else {
				r, changed = insert(r, hash, key, value, hasher, f)
			}
			if !changed {
				return tree, false
			}
			return &branch[K, V]{tree.prefix, tree.branchBit, l, r}, true
		}

		prefix = tree.prefix

	default:
		panic("???")
	}

	newLeaf, _ := insert(nil, hash, key, value, nil, nil)
	return join(hash, prefix, newLeaf, tree), true
}

func remove[K, V any](tree node[K, V], hash keyt, key K, hasher immutable.Hasher[K]) node[K, V] {
	if tree == nil {
		return tree
	}

	switch tree := tree.(type) {
	case *leaf[K, V]:
		if tree.key == hash {
			newLeaf := &leaf[K, V]{tree.key, nil}
			// Copy all pairs that do not match the key
			for _, pr := range tree.values {
				if !hasher.Equal(key, pr.key) {
					newLeaf.values = append(newLeaf.values, pr)
				}
			}

			if len(newLeaf.values) == 0 {
				return nil
			}

			return newLeaf
		}
	case *branch[K, V]:
		if tree.match(hash) {
			left, right := tree.left, tree.right
			if zeroBit(hash, tree.branchBit) {
				left = remove(left, hash, key, hasher)
			} else {
				right = remove(right, hash, key, hasher)
			}
			return br(tree.prefix, tree.branchBit, left, right)
		}
	default:
		panic("???")
	}

	return tree
}

// If the returned flag is true, a and b represent equal trees
func merge[K, V any](a, b node[K, V], hasher immutable.Hasher[K], f mergeFunc[V]) (node[K, V], bool) {
	// Cheap pointer-equality
	if a == b {
		return a, true
	} else if a == nil {
		return b, false
	} else if b == nil {
		return a, false
	}

	// Check if either a or b is a leaf
	lf, isLeaf := a.(*leaf[K, V])
	other := b
	if !isLeaf {
		lf, isLeaf = b.(*leaf[K, V])
		other = a
	}

	if isLeaf {
		originalOther := other
		for _, pr := range lf.values {
			other, _ = insert(other, lf.key, pr.key, pr.value, hasher, f)
		}

		if oLf, oIsLeaf := other.(*leaf[K, V]); oIsLeaf &&
			other == originalOther &&
			len(lf.values) == len(oLf.values) {
			// Since the other tree is also a leaf, and it did not change as a
			// result of inserting our values, and we did not start out with a
			// fewer number of key-value pairs than the other leaf, the two
			// leaves were (and are still) equal.
			return a, true
		}

		return other, false
	}

	// Both a and b are branches
	s, t := a.(*branch[K, V]), b.(*branch[K, V])
	if s.branchBit == t.branchBit && s.prefix == t.prefix {
		l, leq := merge(s.left, t.left, hasher, f)
		r, req := merge(s.right, t.right, hasher, f)
		if leq && req {
			return s, true
		} else if l == s.left && r == s.right {
			return s, false
		} else if l == t.left && r == t.right {
			return t, false
		}

		return &branch[K, V]{s.prefix, s.branchBit, l, r}, false
	}

	if s.branchBit > t.branchBit {
		s, t = t, s
	}

	if s.branchBit < t.branchBit && s.match(t.prefix) {
		// s contains t
		l, r := s.left, s.right
		if zeroBit(t.prefix, s.branchBit) {
			l, _ = merge(l, node[K, V](t), hasher, f)
			if l == s.left {
				return s, false
			}
		} else {
			r, _ = merge(r, node[K, V](t), hasher, f)
			if r == s.right {
				return s, false
			}
		}
		return &branch[K, V]{s.prefix, s.branchBit, l, r}, false
	} else {
		// prefixes disagree
		return join(s.prefix, t.prefix, node[K, V](s), node[K, V](t)), false
	}
	// NOTE: The implementation of this function is complex because it is
	// performance critical, and since the performance does not rely only on
	// the implementation within this function. Using shared subtrees speeds
	// up future merge/equal operations on the result, which is important.
	// The implementation does not (yet) produce a result that shares maximally
	// with one of the input trees. Consider `merge(s, t) = t'`:
	//         s         t          t'
	//        / \      /  \       /  \
	//       0  a     c    b     c    a
	//         / \   / \  / \   / \  / \
	//        1  3  0  2 1  3  0  2 1  3
	// The merge of the leaf `0` and `c` returns `c` because it is a superset of
	// the leaf. However, the merge of `a` and `b` returns `a` because we prefer
	// the left subtree over the right (both `a` and `b` are valid return values
	// as the subtrees are equal). Since `t` is not the branch `(c, a)`, we
	// return a new branch `t'` when we could have just returned `t`.
	// Note also that `merge(t, s) = t`.
}

type cmpFunc[V any] func(a, b V) bool

func equal[K, V any](a, b node[K, V], hasher immutable.Hasher[K], f cmpFunc[V]) bool {
	if a == b {
		return true
	} else if a == nil || b == nil {
		return false
	}

	switch a := a.(type) {
	case *leaf[K, V]:
		b, ok := b.(*leaf[K, V])
		if !ok || len(a.values) != len(b.values) {
			return false
		}

	FOUND:
		for _, apr := range a.values {
			for _, bpr := range b.values {
				if hasher.Equal(apr.key, bpr.key) {
					if !f(apr.value, bpr.value) {
						return false
					}

					continue FOUND
				}
			}

			// a contained a key that b did not
			return false
		}

		return true

	case *branch[K, V]:
		b, ok := b.(*branch[K, V])
		if !ok {
			return false
		}

		return a.prefix == b.prefix && a.branchBit == b.branchBit &&
			equal(a.left, b.left, hasher, f) && equal(a.right, b.right, hasher, f)

	default:
		panic("???")
	}
}
