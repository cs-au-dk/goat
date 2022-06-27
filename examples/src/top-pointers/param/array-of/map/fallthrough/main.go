package main

type A interface {
	Do()
}

func f(a [100]map[int]int) {
}

func main() {
	f([100]map[int]int{{}, make(map[int]int)})
}
