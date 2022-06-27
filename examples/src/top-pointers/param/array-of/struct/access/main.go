package main

func f(a [100]struct{ f int }) {
	_ = a[10].f
}

func main() {
	f([100]struct{ f int }{{10}, {20}})
}
