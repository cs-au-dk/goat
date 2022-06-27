package main

func f(s interface{}) {
	x := s.(*[10]int)
	x[0] = 20
}

func g() {
	x := [10]int{}
	f(&x)
}

func main() {
	g()
}
