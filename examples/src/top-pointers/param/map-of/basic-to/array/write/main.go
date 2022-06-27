package main

func f(a map[int][2]int) {
	a[10] = [2]int{3, 3}
	x := a[10]
	_ = a[10][1]
	_ = x[1]
}

func main() {
	f(make(map[int][2]int))
}
