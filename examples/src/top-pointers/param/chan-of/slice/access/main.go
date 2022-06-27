package main

func f(a chan []int) {
	_ = (<-a)[10]
}

func main() {
	f(make(chan []int))
}
