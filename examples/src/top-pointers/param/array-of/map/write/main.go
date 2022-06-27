package main

type A interface {
	Do()
}

func f(a [100]map[int]int) {
	a[0][1] = 2
}

func main() {
	f([100]map[int]int{{5: 10}, make(map[int]int)})
}
