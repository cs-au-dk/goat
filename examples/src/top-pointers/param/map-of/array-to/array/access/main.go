package main

func f(a map[[100]int][100]int) {
	_ = a[[100]int{}][1]
}

func main() {
	f(make(map[[100]int][100]int))
}
