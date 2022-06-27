package main

func f(s []int) {
	println(s[1])
}

func g() {
	f(make([]int, 0))
}

func main() {
	g()
}
