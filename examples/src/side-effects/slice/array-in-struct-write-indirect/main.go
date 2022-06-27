package main

func f(s *[100]int) {
	s[99] = 20
}

func g() {
	f(&((&struct{ x [100]int }{[100]int{}}).x))
}

func main() {
	g()
}
