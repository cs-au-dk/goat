package main

func f(a [2]chan int) {
	lookup := a[1]
	lookup <- 10
}

func main() {
	f([2]chan int{make(chan int, 1), make(chan int)})
	f([2]chan int{make(chan int), make(chan int)})
}
