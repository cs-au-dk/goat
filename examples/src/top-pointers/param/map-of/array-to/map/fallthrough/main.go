package main

func f(a chan map[int]int) {
}

func main() {
	f(make(chan map[int]int, 1))
}
