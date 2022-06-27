package main

func f(a chan struct{ f int }) {
	_ = (<-a).f
}

func main() {
	f(make(chan struct{ f int }))
}
