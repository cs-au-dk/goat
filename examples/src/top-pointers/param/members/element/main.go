package main

type A struct {
	x int
}

func f(x, y *int) {
	*x = 2
	*y = 3
}

var n = 2

func main() {
	a := make([]A, n)
	b := [2]int{1, 2}
	f(&a[0].x, &b[0])
}
