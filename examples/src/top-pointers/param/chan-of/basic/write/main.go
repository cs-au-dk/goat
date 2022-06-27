package main

func f(a chan int) {
	a = make(chan int)
	a = make(chan int)
}

func main() {
	f(make(chan int))
}
