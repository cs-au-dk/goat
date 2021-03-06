// Command parallel-recursive-fibonacci is a recursive fibonacci which spawns a
// new goroutine per fib call.
package main

// GoLive: replaced fmt.Println with println

// import "fmt"

func main() {
	ch := make(chan int)
	go fib(10, ch)
	println(<-ch)
}

func fib(n int, ch chan<- int) {
	if n <= 1 {
		ch <- n
		return
	}
	ch1 := make(chan int, 2)
	go fib(n-1, ch1)
	go fib(n-2, ch1)
	ch <- <-ch1 + <-ch1
}
