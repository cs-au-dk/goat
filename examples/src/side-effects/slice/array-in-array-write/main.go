package main

func f(s *[100][100]int) {
	s[10][20] = 10
}

func g() {
	f(&[100][100]int{})
}

func main() {
	g()
}
