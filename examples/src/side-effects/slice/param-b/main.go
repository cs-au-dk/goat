package main

func f(s []*int) {
	s[1] = new(int)
}

func g() {
	f(make([]*int, 0))
}

func main() {
	g()
}
