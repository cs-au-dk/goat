package main

type A interface {
	Do()
}

type a struct{}
type b struct{}

func (a) Do() { println("ho") }
func (b) Do() { println("hi") }

func f(a [100]A) {
	a[1].Do()
	a[1] = b{}
	a[2] = &b{}
	a[2].Do()
}

func main() {
	f([100]A{a{}, &a{}})
}
