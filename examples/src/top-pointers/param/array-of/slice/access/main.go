package main

func f(a [100][]int) {
	_ = a[10][10] + a[10][10]
}

func main() {
	f([100][]int{})
}
