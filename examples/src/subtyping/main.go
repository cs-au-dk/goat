package main

import "fmt"

type A struct{}
type B struct{ A }
type C interface {
	doThings()
}

func (A) doThings() {
	fmt.Println("A")
}

func (B) doThings() {
	fmt.Println("B")
}

func doThings(a C) {
	switch a.(type) {
	case A:
		a.doThings()
	case B:
		a.doThings()
	}
}

func main() {
	doThings(A{})
	doThings(B{})
}
