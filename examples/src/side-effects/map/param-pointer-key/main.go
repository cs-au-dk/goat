package main

func f(h map[*int]struct{}) {
	h[new(int)] = struct{}{}
}

func g() {
	f(map[*int]struct{}{
		(new(int)): {},
	})
}

func main() {
	g()
}
