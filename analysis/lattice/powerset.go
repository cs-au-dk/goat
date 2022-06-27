package lattice

import (
	"fmt"
	"strings"
)

type set = map[interface{}]bool

type Powerset struct {
	lattice
	bot *Set
	top *Set

	dom set
}

func (latticeFactory) Powerset(dom set) *Powerset {
	p := new(Powerset)
	p.dom = make(set)
	for x := range dom {
		p.dom[x] = true
	}

	return p
}

func (latticeFactory) PowersetVariadic(dom ...interface{}) *Powerset {
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

func (p *Powerset) Top() Element {
	if p.top == nil {
		p.top = new(Set)
		p.top.lattice = p
		p.top.set = p.dom
	}
	return *p.top
}

func (p *Powerset) Bot() Element {
	if p.bot == nil {
		p.bot = new(Set)
		p.bot.lattice = p
		p.bot.set = make(set)
	}
	return *p.bot
}

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

func (p *Powerset) Domain() set {
	return p.dom
}

func (p *Powerset) Contains(x interface{}) bool {
	_, ok := p.dom[x]
	return ok
}

func (p *Powerset) Extend(x interface{}) {
	p.dom[x] = true
}
