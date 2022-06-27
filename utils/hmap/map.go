package hmap

import "Goat/utils"

// A simple implementation of a mutable hash map.
// Useful when we cannot use Go's maps directly, and we want to avoid the
// overhead of using immutable maps.

// Uses linked lists to resolve hash collisions.

type node[K, V any] struct {
	key   K
	value V
	next  *node[K, V]
}

type Map[K, V any] struct {
	hasher utils.Hasher[K]
	mp     map[uint32]*node[K, V]
}

// Order of V and K are swapped since K can be inferred by the argument.
func NewMap[V, K any](hasher utils.Hasher[K]) *Map[K, V] {
	return &Map[K, V]{
		hasher: hasher,
		mp:     make(map[uint32]*node[K, V]),
	}
}

func (m *Map[K, V]) Set(key K, value V) {
	h := m.hasher.Hash(key)
	if snode, found := m.mp[h]; !found {
		m.mp[h] = &node[K, V]{key, value, nil}
	} else {
		for {
			if m.hasher.Equal(key, snode.key) {
				snode.value = value
				return
			}

			if next := snode.next; next == nil {
				// Hash collision :(
				snode.next = &node[K, V]{key, value, nil}
				return
			} else {
				snode = next
			}
		}
	}
}

func (m *Map[K, V]) GetOk(key K) (res V, ok bool) {
	for node := m.mp[m.hasher.Hash(key)]; node != nil; node = node.next {
		if m.hasher.Equal(key, node.key) {
			return node.value, true
		}
	}

	return
}

func (m *Map[K, V]) Get(key K) V {
	v, _ := m.GetOk(key)
	return v
}
