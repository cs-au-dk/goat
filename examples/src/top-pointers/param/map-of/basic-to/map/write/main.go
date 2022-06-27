package main

func f(a chan map[int]int) {
	a = make(chan map[int]int)
	_ = (<-a)[0]
}

func main() {
	a := make(chan map[int]int)
	a <- make(map[int]int)
	f(a)
}
