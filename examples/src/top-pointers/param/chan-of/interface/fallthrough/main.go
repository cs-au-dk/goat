package main

type A interface {
	Do()
}

type a struct{}

func (a) Do() {}

func f(a chan A) {
}

func main() {
	f(make(chan A))
}
