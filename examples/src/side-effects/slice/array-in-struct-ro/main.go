package main

func f(s *struct{ x [100]int }) {
	println(s.x[10])
}

func g() {
	f(&struct{ x [100]int }{[100]int{}})
}

func main() {
	g()
}
