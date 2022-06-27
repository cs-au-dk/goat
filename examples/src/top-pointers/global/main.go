package main

var x int

func f(y *int) {
	*y = 10

	ch := make(chan int, 1)
	if &x == y {
		ch <- 10 //@ releases
	}
}

func main() {
	f(&x)
}
