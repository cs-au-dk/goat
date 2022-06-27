package main

func f(s []int) {
	s[1] = 10
}

func g() {
	f(make([]int, 0))
}

func main() {
	g()
}
