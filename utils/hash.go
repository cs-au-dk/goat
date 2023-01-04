package utils

import (
	"reflect"

	"github.com/benbjohnson/immutable"
)

type (
	// Hashable is implemented by all hashable types.
	Hashable interface {
		Hash() uint32
	}
	// HashableEq is implemented by all hashable types that can be compared for equality.
	HashableEq[T any] interface {
		Hashable
		Equal(T) bool
	}

	// hashableHasher is a hasher for hashable and equality comparable entities.
	hashableHasher[T HashableEq[T]] struct{}
)

// Equal checks that two hashable entities a and b are equal.
func (hashableHasher[T]) Equal(a, b T) bool { return a.Equal(b) }

// Hash computes the uint32 hash of hashable entity a.
func (hashableHasher[T]) Hash(a T) uint32 { return a.Hash() }

// HashableHasher is a generic hasher factory of hashable and equality comparable entities.
func HashableHasher[T HashableEq[T]]() immutable.Hasher[T] { return hashableHasher[T]{} }

// NewImmMap creates an immutable map where the keys must be hashable and equality comparable.
func NewImmMap[K HashableEq[K], V any]() *immutable.Map[K, V] {
	return immutable.NewMap[K, V](HashableHasher[K]())
}

// PointerHasher is a generic hasher for pointer-like values.
type PointerHasher[T any] struct{}

// Hash computes the uint32 hash of hashable pointer v.
func (PointerHasher[T]) Hash(v T) uint32 {
	// Use reflection to get a uintptr value
	p := reflect.ValueOf(v).Pointer()
	return uint32(p ^ (p >> 32))
}

// Equal checks equality between two hashable pointers.
func (PointerHasher[T]) Equal(a, b T) bool {
	return any(a) == any(b)
}

var _ immutable.Hasher[any] = PointerHasher[any]{}

// HashCombine uses the C++ boost algorithm for combining multiple hash values.
func HashCombine(hs ...uint32) (seed uint32) {
	for _, v := range hs {
		seed = v + 0x9e3779b9 + (seed << 6) + (seed >> 2)
	}

	return
}
