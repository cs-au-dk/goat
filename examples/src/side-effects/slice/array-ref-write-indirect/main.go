package main

func f(s *[100]int) {
	x := &s[1]
	*x = 10
}

func g() {
	f(&[100]int{})
}

func main() {
	g()
}
