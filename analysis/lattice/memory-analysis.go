package lattice

import (
	loc "Goat/analysis/location"
	"fmt"
)

// Map methods
func (w Memory) Size() int {
	return w.values.Size()
}

// Returns the size in terms of non-bottom locations
func (w Memory) EffectiveSize() (count int) {
	w.ForEach(func(al loc.AddressableLocation, av AbstractValue) {
		if !av.IsBot() {
			count++
		}
	})

	return
}

func (m Memory) valueHeight(key loc.AddressableLocation, v AbstractValue) int {
	h := v.Height()
	if aLoc, isAllocSite := key.(loc.AllocationSiteLocation); isAllocSite {
		if elem, found := m.allocs.Lookup(aLoc); found {
			h += elem.Height()
		}
	}
	return h
}

func (m Memory) Height() (h int) {
	m.values.ForEach(func(key loc.AddressableLocation, v AbstractValue) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println(v)
				panic(err)
			}
		}()
		h += m.valueHeight(key, v)
	})
	return
}

func (m Memory) MaxElementHeight() (h int) {
	m.values.ForEach(func(key loc.AddressableLocation, v AbstractValue) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println(v)
				panic(err)
			}
		}()
		vh := m.valueHeight(key, v)
		if h < vh {
			h = vh
		}
	})
	return
}

// Get the difference in height between all elements of two abstract memories.
func (m1 Memory) HeightDiff(m2 Memory) (h int) {
	m2.ForEach(func(al loc.AddressableLocation, av AbstractValue) {
		av2, found := m1.Get(al)
		if !found {
			h += m1.valueHeight(al, av)
		}
		h1, h2 := m1.valueHeight(al, av), m2.valueHeight(al, av2)
		if h2 < h1 {
			h += h1 - h2
		}
	})

	return
}

// Check the differences between two abstract memories
func (m1 Memory) Difference(m2 Memory) (diff Memory) {
	diff = Create().Element().Memory()

	// update := func(l loc.AddressableLocation, v1, v2 AbstractValue) {
	// 	b = b.Update(l, v1)
	// 	a = a.Update(l, v2)
	// }

	// m1.ForEach(func(al loc.AddressableLocation, av AbstractValue) {
	// 	av2, found := m2.Get(al)
	// 	switch {
	// 	case !found:
	// 		update(al, av, Consts().BotValue())
	// 	case !av.eq(av2):
	// 		update(al, av, av2)
	// 	}
	// })
	m2.ForEach(func(al loc.AddressableLocation, av AbstractValue) {
		av2, _ := m1.Get(al)
		vdiff, relevant := av2.Difference(av)
		if relevant {
			diff = diff.Update(al, vdiff)
		}
	})

	return
}
