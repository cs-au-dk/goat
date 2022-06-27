package main

type A struct {
	a int
}

type B struct {
	b int
}

type C interface {
	doThings() bool
}

func (a A) doThings() bool {
	a.a = 5
	return true
}

func (b *B) doThings() bool {
	b.b = 6
	return false
}

func doThings(a C) {
	a.doThings()
}

func main() {
	if true {
		doThings(A{})
	} else {
		doThings(new(B))
	}
}
