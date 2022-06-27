package main

import "fmt"

func forkedReturn() (x int) {
	if true {
		x = 5
		return x
	} else {
		x = 6
		return x
	}
}

func i() { main() }
func h() { main() }

func g(f func(), x chan int) {
	f()
}

func main() {
	var x *int
	var y int
	x = new(int)
	x = &y
	fmt.Println(x)
	g(i, make(chan int))
	g(h, make(chan int))
}
