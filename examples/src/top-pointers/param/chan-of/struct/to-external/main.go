package main

type ch = chan struct{}

func external(ch)

func f(ch ch) {
	external(ch)
}

func main() {
	f(make(ch))
}
