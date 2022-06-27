package main

func f(a [100]func()) {
	a[0]()
}

func main() {
	f([100]func(){func() { println("1") }, func() { println("2") }})
}
