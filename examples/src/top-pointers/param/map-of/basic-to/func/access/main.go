package main

func f(a chan func()) {
	(<-a)()
}

func main() {
	a := make(chan func(), 1)
	a <- func() { println("a") }
	f(a)
}
