package main

func f(h map[int]map[int]struct{}) {
	h[10][10] = struct{}{}
}

func g() {
	f(map[int]map[int]struct{}{
		0: {},
	})
}

func main() {
	g()
}
