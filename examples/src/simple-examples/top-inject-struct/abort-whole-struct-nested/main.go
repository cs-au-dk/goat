package main

type B struct {
	z int
	y chan int
}

type A struct {
	x int
	b B
}

func f(a *A) {
	a.x = 10
	a.b.z = 20
}

func g() {
	a := &A{b: B{y: make(chan int, 1)}}

	f(a)
	a.b.y <- 10
}

func main() {
	g()
}
