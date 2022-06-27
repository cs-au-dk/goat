package main

type A struct {
	x *A
	y struct{ z int }
}

func F(a *A) {
	a.y.z = 20
}

func main() {
	x := A{x: new(A)}
	x.x.y.z = 10

	F(x.x)
}
