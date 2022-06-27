package main

type m = map[int]int

// Trick f into analyzing g since s contains a concurrency primitive
type s struct { ch chan int }
func (*s) g() m {
	return make(m)
}

func f(x m) {
	_ = x[10]
	(*s)(nil).g()
}

func main() {
	f((*s)(nil).g())
}
