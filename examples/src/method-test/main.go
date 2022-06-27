package main

type A struct{}
type B struct{ A }
type C interface {
	doThings()
}

func (A) doThings() {
	ch := make(chan int)
	println("A")
	<-ch
}

func (B) doThings() {
	println("B")
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
