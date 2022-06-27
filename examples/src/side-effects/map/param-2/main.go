package main

func f(h map[int]struct{}) {
	h[10] = struct{}{}
}

func main() {
	f(make(map[int]struct{}))
	f(map[int]struct{}{})
}
