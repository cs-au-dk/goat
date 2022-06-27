package main

type E interface {
	f()
}

type I interface {
	E
}

type s struct { E }
func (s) f() {}
func (s) g() {}

func f(e E) {
	e.f() // Force swap
	if e != nil {
		make(chan int) <- 0 //@ blocks
	}
}

func main() {
	var i I = s{}
	f(i)
}
