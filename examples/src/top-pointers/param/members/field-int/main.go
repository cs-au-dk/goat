package main

type A struct {
	x int
}

func f(x *int) {
	*x = 10 // force swap
	if x != nil {
		make(chan int) <- 0 //@ blocks
	}
}

func main() {
	var a A
	f(&a.x)
}
