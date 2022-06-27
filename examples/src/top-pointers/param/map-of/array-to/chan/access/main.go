package main

func f(a chan chan int) {
	<-<-a
}

func main() {
	f(make(chan chan int))
}
