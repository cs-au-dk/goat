package main

func f(s *[100][100]int) {
	println(s[10][20])
}

func g() {
	f(&[100][100]int{})
}

func main() {
	g()
}
