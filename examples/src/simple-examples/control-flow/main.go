package main

var ubool = []bool{false, true}[0]
var uint = 0
var intptr *int

func H() {
	switch uint {
	case 0:  println("A")
	case 1:  println("B")
	case 2:  println("C")
	default: println("D")
	}
}

func G(x *int) {
	intptr = x
	H()
}

type HasF interface {
	F()
}

type A struct {}
func (*A) F() { G(new(int)) }
type B struct {}
func (*B) F() { G(new(int)) }
type C struct {}
func (*C) F() { G(new(int)) }
type D struct {}
func (*D) F() { G(new(int)) }


func main() {
	if ubool { uint = 1 }

	var itf HasF
	switch uint {
	case 0: itf = &A{}
	case 1: itf = &B{}
	case 2: itf = &C{}
	default: itf = &D{}
	}

	itf.F()
}
