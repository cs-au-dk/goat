package tree

import (
	"math/rand"
	"testing"

	"github.com/benbjohnson/immutable"
)

var intHasher = immutable.NewHasher[any](int(0))
var uint32Hasher = immutable.NewHasher[any](uint32(0))

type tree = Tree[any, any]

func testLookup(t *testing.T, tree tree, key interface{}, expectFound bool, expectVal interface{}) {
	val, found := tree.Lookup(key)
	if found != expectFound {
		if found {
			t.Error("Expected miss for", key)
		} else {
			t.Error("Expected hit for", key)
		}
	}

	if val != expectVal {
		t.Errorf("Lookup(%v) = %v, expected: %v", key, val, expectVal)
	}
}

func mkTest(t *testing.T) (
	func(tree tree, key, val interface{}),
	func(tree, interface{}),
) {
	return func(tree tree, key, expectVal interface{}) {
			if val, found := tree.Lookup(key); found {
				if val != expectVal {
					t.Errorf("Lookup(%v) = %v, expected: %v", key, val, expectVal)
				}
			} else {
				t.Error("Expected hit for", key)
			}
		}, func(tree tree, key interface{}) {
			if _, found := tree.Lookup(key); found {
				t.Fatal("Expected miss for", key)
			}
		}
}

func TestEmpty(t *testing.T) {
	tree := NewTree[any, any](intHasher)
	testLookup(t, tree, 0, false, nil)
}

func itfEq(a, b interface{}) bool {
	return a == b
}

type memHasher struct {
	mem   map[int]uint32
	limit int
}

func (m memHasher) Hash(i interface{}) uint32 {
	x := i.(int)
	if v, ok := m.mem[x]; ok {
		return v
	}
	h := uint32(rand.Intn(m.limit))
	m.mem[x] = h
	return h
}
func (m memHasher) Equal(a, b interface{}) bool {
	return a == b
}
func mkMemHasher(limit int) memHasher {
	return memHasher{make(map[int]uint32), limit}
}

func TestSameKey(t *testing.T) {
	for _, hasher := range []immutable.Hasher[any]{intHasher, badHasher{}} {
		hit, miss := mkTest(t)
		tree0 := NewTree[any, any](hasher)
		tree1 := tree0.Insert(0, "v1")
		tree2 := tree1.Insert(0, "v2")

		miss(tree0, 0)
		hit(tree1, 0, "v1")
		hit(tree2, 0, "v2")

		if tree1.Equal(tree2, itfEq) {
			t.Error(tree1, "should not equal", tree2)
		}
	}
}

type badHasher struct{}

func (badHasher) Hash(interface{}) uint32     { return 0 }
func (badHasher) Equal(a, b interface{}) bool { return a == b }

func TestHashCollision(t *testing.T) {
	hit, miss := mkTest(t)
	tree0 := NewTree[any, any](badHasher{})
	tree1 := tree0.Insert(1, "v1")
	tree2 := tree1.Insert(2, "v2")

	miss(tree0, 1)
	miss(tree0, 2)

	hit(tree1, 1, "v1")
	miss(tree1, 2)

	hit(tree2, 1, "v1")
	hit(tree2, 2, "v2")
}

func TestDiffKey(t *testing.T) {
	hit, _ := mkTest(t)
	tree := NewTree[any, any](intHasher).Insert(0, "v1").Insert(1, "v2")
	hit(tree, 0, "v1")
	hit(tree, 1, "v2")

	tree = tree.Insert(2, "v3")
	hit(tree, 0, "v1")
	hit(tree, 1, "v2")
	hit(tree, 2, "v3")
}

func TestManyInsert(t *testing.T) {
	iterations := 100
	N := 100

	for iter := 0; iter < iterations; iter++ {
		tree := NewTree[any, any](uint32Hasher)

		var keys []uint32
		for i := 0; i < N; i++ {
			k := rand.Uint32()
			keys = append(keys, k)
			tree = tree.Insert(k, k)
		}

		rand.Shuffle(N, func(i, j int) {
			keys[i], keys[j] = keys[j], keys[i]
		})

		for _, k := range keys {
			testLookup(t, tree, k, true, k)
		}
	}
}

func TestHistory(t *testing.T) {
	hit, miss := mkTest(t)
	N := 100

	for _, hasher := range []immutable.Hasher[any]{intHasher, mkMemHasher(N / 5)} {
		tree := NewTree[any, any](hasher)
		history := []Tree[any, any]{tree}

		for i := 0; i < N; i++ {
			tree = tree.Insert(i, i)
			history = append(history, tree)
		}

		for vidx, tree := range history {
			for i := 0; i < N; i++ {
				if vidx <= i {
					miss(tree, i)
				} else {
					hit(tree, i, i)
				}
			}
		}
	}
}

func max(a, b interface{}) (interface{}, bool) {
	x, y := a.(int), b.(int)
	if x == y {
		return x, true
	}
	if x > y {
		return x, false
	} else {
		return y, false
	}
}

func TestSimpleMerge(t *testing.T) {
	hit, _ := mkTest(t)
	for _, hasher := range []immutable.Hasher[any]{intHasher, badHasher{}, mkMemHasher(2)} {
		a := NewTree[any, any](hasher).Insert(0, 1).Insert(1, 1)
		b := NewTree[any, any](hasher).Insert(1, 2).Insert(2, 2)

		check := func(tree Tree[any, any]) {
			hit(tree, 0, 1)
			hit(tree, 1, 2)
			hit(tree, 2, 2)

			if sz := tree.Size(); sz != 3 {
				t.Error("Wrong size:", sz)
			}
		}

		check(a.Merge(b, max))
		check(b.Merge(a, max))
	}
}

func TestMergeWithEmpty(t *testing.T) {
	a := NewTree[any, any](intHasher).Insert(0, 0)
	a.Merge(NewTree[any, any](intHasher), max)
}

func TestPointerEqualityAfterMerge(t *testing.T) {
	a, b := NewTree[any, any](intHasher), NewTree[any, any](intHasher)
	for i := 0; i < 4; i++ {
		a = a.Insert(i, i)
		if i < 3 {
			b = b.Insert(i, i)
		}
	}

	c := a.Merge(b, func(x, y interface{}) (interface{}, bool) {
		return x, x == y
	})

	if !c.Equal(a, itfEq) {
		t.Fatalf("Equality or Merge is buggy. %v should be equal to %v", c, a)
	}

	if c.root != a.root {
		// Since `a` is a superset of `b`, we should be able to retain the
		// identity of the root.
		t.Errorf("Expected %p to be %p", c.root, a.root)
		t.Log(c.root.(*branch[any, any]).left)
		t.Log(a.root.(*branch[any, any]).left)
	}
}


func TestManyMerge(t *testing.T) {
	hit, _ := mkTest(t)
	iterations := 100
	N := 100

	for iter := 0; iter < iterations; iter++ {
		for _, hasher := range []immutable.Hasher[any]{intHasher, mkMemHasher(N / 5)} {
			a, b := NewTree[any, any](hasher), NewTree[any, any](hasher)

			mp := make([]int, 2*N)
			for i := 0; i < 2*N; i++ {
				v1, v2 := rand.Int(), rand.Int()
				if i < N {
					mx, _ := max(v1, v2)
					mp[i] = mx.(int)
					a = a.Insert(i, v1)
					b = b.Insert(i, v2)
				} else if i < 3*N/2 {
					mp[i] = v1
					a = a.Insert(i, v1)
				} else {
					mp[i] = v2
					b = b.Insert(i, v2)
				}
			}

			merged := a.Merge(b, max)
			for k, v := range mp {
				hit(merged, k, v)
			}

			reconstructed := NewTree[any, any](hasher)
			for k, v := range mp {
				reconstructed = reconstructed.Insert(k, v)
			}

			if !reconstructed.Equal(merged, itfEq) {
				t.Fatal("Expected", reconstructed, "to equal", merged)
			}
		}
	}
}

func TestRemove(t *testing.T) {
	hit, miss := mkTest(t)
	iterations := 100
	N := 100
	N_remove := 20

	for iter := 0; iter < iterations; iter++ {
		tree := NewTree[any, any](uint32Hasher)

		var keys []uint32
		for i := 0; i < N; i++ {
			k := rand.Uint32()
			keys = append(keys, k)
			tree = tree.Insert(k, k)
		}

		rand.Shuffle(N, func(i, j int) {
			keys[i], keys[j] = keys[j], keys[i]
		})

		removed := keys[:N_remove]
		for _, k := range removed {
			tree = tree.Remove(k)
		}

		if sz := tree.Size(); sz != N-N_remove {
			t.Error("Expected sz to be", N-N_remove, "was", sz)
		}

		for _, k := range removed {
			miss(tree, k)
		}

		for _, k := range keys[N_remove:] {
			hit(tree, k, k)
		}
	}
}
