package main

func f(a [100]chan int) {
}

func main() {
	f([100]chan int{make(chan int)})
}
