package main

func f(a chan map[int]int) {
	a <- make(map[int]int)
	_ = (<-a)[0]
}

func main() {
	f(make(chan map[int]int, 1))
}
