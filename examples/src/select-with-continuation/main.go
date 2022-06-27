package main

// GoLive: replaced fmt.Println with println

import (
	//"fmt"
)

func main() {

	ch1 := make(chan int)
	ch2 := make(chan int)
	ch3 := make(chan int)

	select {
	case x := <-ch1:
		println("Received x", x)
	case ch2 <- 43:
		println("ok sent")
	case <-ch3:
	default:
		println("asdfsdafsad")
	}
	ch1 <- 32
	println("asdfsadfs")
}
