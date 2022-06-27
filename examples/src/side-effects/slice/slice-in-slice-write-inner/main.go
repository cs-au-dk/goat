package main

func f(s [][]int) {
	s[10][20] = 20
}

func g() {
	f([][]int{
		{},
	})
}

func main() {
	g()
}
