package main

import "time"

func f(a **int) {
	**a = 10
}

func g() {
	var a **int
	select {
	case <-time.After(10 * time.Millisecond):
		a = new(*int)
		*a = new(int)
	default:
		a = new(*int)
		*a = new(int)
	}
	f(a)
}

func main() {
	g()
}
