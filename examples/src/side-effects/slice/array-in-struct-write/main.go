package main

func f(s *struct{ x [100]int }) {
	s.x[99] = 20
}

func g() {
	f(&struct{ x [100]int }{[100]int{}})
}

func main() {
	g()
}
