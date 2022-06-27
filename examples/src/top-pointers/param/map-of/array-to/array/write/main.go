package main

func f(a chan [100]int) {
	a <- [100]int{}
}

func main() {
	f(make(chan [100]int))
}
