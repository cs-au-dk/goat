package main

func f(a chan []int) {
	a <- []int{}
}

func main() {
	f(make(chan []int))
}
