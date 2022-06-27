package main

func f(a chan []int) {
}

func main() {
	f(make(chan []int))
}
