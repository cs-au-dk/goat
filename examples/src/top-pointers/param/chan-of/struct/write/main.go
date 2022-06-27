package main

func f(a chan struct{ f int }) {
}

func main() {
	f(make(chan struct{ f int }))
}
