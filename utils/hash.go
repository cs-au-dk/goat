package utils

import (
	"reflect"
)

// Use the C++ boost algorithm for combining multiple hash values.
func HashCombine(hs ...uint32) (seed uint32) {
	for _, v := range hs {
		seed = v + 0x9e3779b9 + (seed << 6) + (seed >> 2)
	}

	return
}

type PointerHasher struct{}

func (PointerHasher) Hash(v interface{}) uint32 {
	// Use reflection to get a uintptr value
	p := reflect.ValueOf(v).Pointer()
	return uint32(p ^ (p >> 32))
}

func (PointerHasher) Equal(a, b interface{}) bool {
	return a == b
}

type Hashable interface {
	Hash() uint32
}

type Hasher[T any] interface {
	Hash(a T) uint32
	Equal(a, b T) bool
}
