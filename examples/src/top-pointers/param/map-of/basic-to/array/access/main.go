package main

func f(a map[int][100]int) {
	x := a[10]
	_ = a[10][10]
	_ = x[10]
}

func main() {
	f(make(map[int][100]int))
}
