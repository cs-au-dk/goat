package main

type I interface {
	Do()
}
type i1 struct{ x int }
type i2 struct{}

func (e i1) Do() { println(e.x) }
func (*i2) Do()  {}

func f(a I) {
	println(a.(interface{}))

	println(a.(*i1))
}

func main() {
	f(i1{})
	f(&i1{})
	f(&i2{})
}
