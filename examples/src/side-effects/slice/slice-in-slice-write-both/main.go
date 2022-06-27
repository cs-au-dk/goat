package main

func f(s [][]int) {
	s[10] = []int{}
	s[10][20] = 10
}

func g() {
	f([][]int{
		{},
	})
}

func main() {
	g()
}
