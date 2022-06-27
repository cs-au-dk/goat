package main

type A struct {
	x int
	y chan int
}

func f(a *int) {
	*a = 10
}

func g() {
	a := &A{y: make(chan int, 1)}

	f(&a.x)
	a.y <- 10
}

func main() {
	g()
}
