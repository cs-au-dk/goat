package main

type I interface {
	f() int
}

type A struct { }
func (a *A) f() int {
	return 10
}

type B struct {
	A
}

func main() {
	var i I = &B{}
	i.f()
}
