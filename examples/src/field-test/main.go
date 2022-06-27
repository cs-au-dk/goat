package main

type A struct {
	b struct{ x int }
}

func main() {
	var a *A = new(A)

	a.b.x = 10
}
