package main

// Example from CONCUR 14 paper by Giachino et al.
// doi: 10.1007/978-3-662-44584-6_6

// GoLive: This example is nice to analyse with a dynamic analysis frontend
// 		I replaced fmt.Println with println

func fact(n int, r, s chan int) {
	if n == 0 {
		// The analysis does not reach this branch because of precise
		// integer constant propagation and the unsound goro bound.
		m := <-r // @ analysis(true)
		s <- m
		return
	}
	t := make(chan int)
	go fact(n-1, t, s)
	m := <-r
	t <- m * n
}

func main() {
	accumulated, result := make(chan int), make(chan int)
	go fact(3, accumulated, result)
	accumulated <- 1 //@ analysis(true)
	println(<-result)
}
