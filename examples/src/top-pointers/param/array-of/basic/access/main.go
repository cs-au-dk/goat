package main

func f(a [100]int) {
	x := a[10]
	_ = x + x
}

func main() {
	f([100]int{})
}
