package main

func f(a [100][100]int) {
	a[10] = [100]int{10}
	a[10][10] = 5
	_ = a[10]
}

func main() {
	f([100][100]int{})
}
