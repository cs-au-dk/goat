package main

func f(a [100]struct{ f int }) {
}

func main() {
	f([100]struct{ f int }{{10}, {20}})
}
