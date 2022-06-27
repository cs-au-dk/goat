package lattice

import (
	"fmt"
	"strings"
)

type Set struct {
	element

	set set
}

func newSet(lat Lattice) Set {
	lat2 := lat.Powerset()
	e := Set{
		element{lat2},
		make(set),
	}
	return e
}

func (elementFactory) Powerset(lat Lattice) func(set set) Set {
	switch lat := lat.(type) {
	case *Powerset:
		return func(set set) Set {
			el := newSet(lat)

			for x, in := range set {
				if _, legal := lat.dom[x]; legal {
					el.set[x] = in
				} else {
					panic(fmt.Sprintf("Element %s does not belong in sets of %s", x, lat))
				}
			}

			return el
		}
	case *Dropped:
		return elFact.Powerset(lat.Lattice)
	case *Lifted:
		return elFact.Powerset(lat.Lattice)
	default:
		panic("Attempted to create set with non-powerset lattice")
	}
}

func (e Set) String() string {
	strs := []string{}
	for x, contained := range e.set {
		if contained {
			strs = append(strs, fmt.Sprintf("%s", x))
		}
	}

	if len(strs) == 0 {
		return colorize.Element("∅")
	}
	return "{ " + strings.Join(strs, ", ") + " }"
}

func (e1 Set) Eq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "=")
	return e1.eq(e2)
}

func (e1 Set) eq(e2 Element) bool {
	return e1.geq(e2) && e1.leq(e2)
}

func (e1 Set) Geq(e2 Element) (result bool) {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊒")
	return e1.geq(e2)
}

func (e1 Set) geq(e2 Element) (result bool) {
	result = true
	switch e2 := e2.(type) {
	case Set:
		for x, in2 := range e2.set {
			if in1, ok := e1.set[x]; (!in1 || !ok) && in2 {
				return false
			}
		}
		return
	case *DroppedTop:
		return false
	case *LiftedBot:
		return true
	default:
		panic(errInternal)
	}
}

func (e1 Set) Leq(e2 Element) bool {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊑")
	return e1.leq(e2)
}

func (e1 Set) leq(e2 Element) (result bool) {
	switch e2 := e2.(type) {
	case Set:
		result = true
		for x, in1 := range e1.set {
			if in2, ok := e2.set[x]; in1 && (!in2 || !ok) {
				return false
			}
		}
		return true
	case *DroppedTop:
		return true
	case *LiftedBot:
		return false
	default:
		panic(errInternal)
	}
}

func (e1 Set) Join(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊔")
	return e1.join(e2)
}

func (e1 Set) join(e2 Element) Element {
	switch e2 := e2.(type) {
	case Set:
		return e1.MonoJoin(e2)
	case *DroppedTop:
		return e2
	case *LiftedBot:
		return e1
	default:
		panic(errInternal)
	}
}

func (e1 Set) Meet(e2 Element) Element {
	checkLatticeMatch(e1.lattice, e2.Lattice(), "⊓")
	return e1.meet(e2)
}

func (e1 Set) meet(e2 Element) Element {
	switch e2 := e2.(type) {
	case Set:
		return e1.MonoMeet(e2)
	case *DroppedTop:
		return e1
	case *LiftedBot:
		return e2
	default:
		panic(errInternal)
	}
}

func (e Set) Contains(x interface{}) bool {
	contained, ok := e.set[x]
	return contained && ok
}

func (e Set) Add(x interface{}) Set {
	powLat := e.Lattice().Powerset()
	if !powLat.Contains(x) {
		panic(fmt.Sprintf("%s is not part of the domain of powerset lattice:\n%s", x, powLat))
	}
	e2 := newSet(e.lattice)
	for x2, contained := range e.set {
		if contained || x2 == x {
			e2.set[x2] = true
		}
	}
	return e2
}

func (e Set) Remove(x interface{}) Set {
	powLat := e.Lattice().Powerset()
	if !powLat.Contains(x) {
		panic(fmt.Sprintf("%s is not part of the domain of powerset lattice:\n%s", x, powLat))
	}
	e2 := newSet(e.lattice)
	for x2, contained := range e.set {
		if contained && x2 != x {
			e2.set[x2] = true
		}
	}
	return e
}

func (e Set) All() set {
	return e.set
}

func (e Set) Set() Set {
	return e
}

func (e Set) Height() int {
	return e.Size()
}

func (e1 Set) MonoJoin(e2 Set) Set {
	e3 := newSet(e1.lattice)
	for x, contained := range e1.set {
		if contained {
			e3.set[x] = true
		}
	}
	for x, contained := range e2.set {
		if contained {
			e3.set[x] = true
		}
	}
	return e3
}

func (e1 Set) MonoMeet(e2 Set) Set {
	e3 := newSet(e1.lattice)
	for x, in1 := range e1.set {
		if in2, ok := e2.set[x]; in2 && ok {
			e3.set[x] = in1
		}
	}
	return e3
}

func (e1 Set) Size() (size int) {
	for _, contained := range e1.set {
		if contained {
			size++
		}
	}
	return
}
