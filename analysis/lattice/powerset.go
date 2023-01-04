package lattice

import (
	"fmt"
	"strings"
)

// set represents a finite collection of elements.
type set = map[any]bool

// Powerset is a lattice constructed from a given set.
type Powerset struct {
	lattice
	bot *Set
	top *Set

	dom set
}

// Powerset constructs a powerset lattice from a given set of values.
// The set members may have heterogeneous types.
func (latticeFactory) Powerset(dom set) *Powerset {
	p := new(Powerset)
	p.dom = make(set)
	for x := range dom {
		p.dom[x] = true
	}

	return p
}

// PowersetVaradiac is a variadic variant of the Powerset lattice factory method.
// The constructed powerset lattice will contain all elements given to dom.
func (latticeFactory) PowersetVariadic(dom ...any) *Powerset {
	p := new(Powerset)
	p.dom = make(set)
	for _, x := range dom {
		p.dom[x] = true
	}

	return p
}

func (p *Powerset) Powerset() *Powerset {
	return p
}

// Top returns the ⊤ element for the powerset lattice, containing all elements in the set.
func (p *Powerset) Top() Element {
	if p.top == nil {
		p.top = new(Set)
		p.top.lattice = p
		p.top.set = p.dom
	}
	return *p.top
}

// Bot returns the ⊥ element for the powerset lattice, the empty set.
func (p *Powerset) Bot() Element {
	if p.bot == nil {
		p.bot = new(Set)
		p.bot.lattice = p
		p.bot.set = make(set)
	}
	return *p.bot
}

// Eq checks whether another lattice is equal to the subject powerset lattice.
func (l1 *Powerset) Eq(l2 Lattice) bool {
	// First try to get away with referential equality
	if l1 == l2 {
		return true
	}
	switch l2 := l2.(type) {
	case *Powerset:
		for x := range l1.dom {
			if contains, ok := l2.dom[x]; !contains || !ok {
				return false
			}
		}
		for x := range l2.dom {
			if contains, ok := l1.dom[x]; !contains || !ok {
				return false
			}
		}
		return true
	case *Dropped:
		return l1.Eq(l2.Lattice)
	case *Lifted:
		return l1.Eq(l2.Lattice)
	default:
		return false
	}
}

func (p *Powerset) String() string {
	strs := []string{}
	for x := range p.dom {
		strs = append(strs, fmt.Sprintf("%s", x))
	}

	if len(strs) == 0 {
		return colorize.Lattice("℘") + "(" +
			colorize.Lattice("∅") + ")"

	}

	return colorize.Lattice("℘") + "({" + strings.Join(strs, ", ") + "})"
}

// Domain retrieves all the elements of the set from which the powerset is derived.s
func (p *Powerset) Domain() set {
	return p.dom
}

// Contains checks whether an element belongs to the set from which a powerset is derived.
func (p *Powerset) Contains(x any) bool {
	_, ok := p.dom[x]
	return ok
}
