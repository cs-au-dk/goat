package main

func f(s *[10]int) {
	*s = [10]int{}
}

func g() {
	x := [10]int{10}
	f(&x)
}

func main() {
	g()
}
