package main

func f(a [100]func()) {
}

func main() {
	f([100]func(){func() { println("1") }, func() { println("2") }})
}
