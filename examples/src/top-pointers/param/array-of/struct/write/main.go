package main

func f(a [100]struct{ f int }) {
	a[10].f = 20
}

func main() {
	f([100]struct{ f int }{{10}, {20}})
}
