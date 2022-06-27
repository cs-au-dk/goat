package main

type A interface {
	Do()
}

type a struct{}

func (a) Do() {}

func f(a [100]A) {
}

func main() {
	f([100]A{a{}, &a{}})
}
