package main

func f(a [100]int) {
	a[10] = 50
}

func main() {
	f([100]int{})
}
