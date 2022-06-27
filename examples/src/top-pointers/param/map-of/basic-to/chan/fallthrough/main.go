package main

func f(a chan chan int) {
	x := a
	_ = x
}

func main() {
	f(make(chan chan int))
}
