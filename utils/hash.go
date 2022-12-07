package utils

import (
	"reflect"

	"github.com/benbjohnson/immutable"
)

// Use the C++ boost algorithm for combining multiple hash values.
func HashCombine(hs ...uint32) (seed uint32) {
	for _, v := range hs {
		seed = v + 0x9e3779b9 + (seed << 6) + (seed >> 2)
	}

	return
}

type PointerHasher[T any] struct{}

func (PointerHasher[T]) Hash(v T) uint32 {
	// Use reflection to get a uintptr value
	p := reflect.ValueOf(v).Pointer()
	return uint32(p ^ (p >> 32))
}

func (PointerHasher[T]) Equal(a, b T) bool {
	return any(a) == any(b)
}

var _ immutable.Hasher[any] = PointerHasher[any]{}

type Hashable interface {
	Hash() uint32
}

type HashableEq[T any] interface {
	Hashable
	Equal(T) bool
}

type hashableHasher[T HashableEq[T]] struct{}

func (hashableHasher[T]) Equal(a, b T) bool { return a.Equal(b) }
func (hashableHasher[T]) Hash(a T) uint32   { return a.Hash() }

func HashableHasher[T HashableEq[T]]() immutable.Hasher[T] { return hashableHasher[T]{} }

func NewImmMap[K HashableEq[K], V any]() *immutable.Map[K, V] {
	return immutable.NewMap[K, V](HashableHasher[K]())
}
