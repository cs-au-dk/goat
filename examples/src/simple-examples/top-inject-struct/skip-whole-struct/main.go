package main

type A struct {
	x int
	y chan int
}

func f(a *A) {
	a.x = 10
}

func g() {
	a := &A{y: make(chan int, 1)}

	f(a)
	a.y <- 10
}

func main() {
	g()
}
