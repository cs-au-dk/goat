package main

func f(s [][]int) {
	s[10] = []int{}
}

func g() {
	f([][]int{
		{},
	})
}

func main() {
	g()
}
