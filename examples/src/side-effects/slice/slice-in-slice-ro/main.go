package main

func f(s [][]int) {
	println(s[10][20])
}

func g() {
	f(make([][]int, 0))
}

func main() {
	g()
}
