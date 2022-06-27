package main

type customInts []int

func f(a *int) {
	*a = 10
}

func main() {
	a := make(customInts, func() int { return 10 }())
	f(&a[1])
}
