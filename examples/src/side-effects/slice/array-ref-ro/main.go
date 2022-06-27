package main

func f(s *[100]int) {
	println(s[1])
}

func g() {
	f(&[100]int{})
}

func main() {
	g()
}
