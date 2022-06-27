package main

func f(a chan [100]int) {
	_ = (<-a)[10]
}

func main() {
	f(make(chan [100]int))
}
