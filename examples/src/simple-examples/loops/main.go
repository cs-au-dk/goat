package main


func f() []int {
	ints := make([]int, 10)
	for i := 0; i < 10; i++ {
		ints[i] = i
	}
	return ints
}

func g(ints []int) {
	for i, x := range ints {
		println(i, x)
	}
}

func main() {
	g(f())
}
