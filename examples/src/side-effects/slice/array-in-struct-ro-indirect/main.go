package main

func f(s *[100]int) {
	println(s[99])
}

func g() {
	f(&((&struct{ x [100]int }{[100]int{}}).x))
}

func main() {
	g()
}
