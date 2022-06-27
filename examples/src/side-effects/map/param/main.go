package main

func f(h map[int]struct{}) {
	h[10] = struct{}{}
}

func g() {
	f(make(map[int]struct{}))
}

func main() {
	g()
}
