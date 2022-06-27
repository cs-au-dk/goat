package main

type A interface {
	doStuff()
}

func f(a A) {
	for i := 0; i < 10; i++ {
		go a.doStuff()
	}
}

func main() {
	f(nil)
}
