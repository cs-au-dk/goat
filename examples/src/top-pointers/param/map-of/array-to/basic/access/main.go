package main

func f(a chan int) {
	<-a
	a <- 10
}

func main() {
	f(make(chan int))
}
