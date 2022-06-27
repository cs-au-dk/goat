package main

func f(s *[100]int) {
	s[1] = 10
}

func g() {
	f(&[100]int{})
}

func main() {
	g()
}
