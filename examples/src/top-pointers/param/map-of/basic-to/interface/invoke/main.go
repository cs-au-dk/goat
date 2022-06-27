package main

type A interface {
	Do()
}

type a struct{}

func (a) Do() {}

func f(a chan A) {
	(<-a).Do()
}

func main() {
	ch := make(chan A, 1)
	ch <- a{}
	f(make(chan A))
}
