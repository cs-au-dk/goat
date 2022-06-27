package main

func f(a [100]func()) {
	a[1] = func() { println("3") }
	a[1]()
}

func main() {
	f([100]func(){func() { println("1") }, func() { println("2") }})
}
