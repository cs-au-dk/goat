package main

type A struct { x int }
func (a A) f() int { return a.x }

type I interface {
	f() int
}

func main() {
	var i I = &A{}
	i.f()
}
