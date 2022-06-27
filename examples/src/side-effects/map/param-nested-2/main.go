package main

func f(h map[int]map[int]int) {
	h[10][10] = 10
}

func g() {
	f(map[int]map[int]int{
		0: {},
		1: {},
	})
	f(map[int]map[int]int{
		2: {},
	})

}

func main() {
	g()
}
