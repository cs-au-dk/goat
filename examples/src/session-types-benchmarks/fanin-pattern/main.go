package main

// GoLive: replaced fmt.Println with println
// Requires some refinement of C to analyse
// Very similar to popl17/fanin except that work is split into work1 and work2

import (
	//"fmt"
)

func work(out chan<- int) {
	for {
		out <- 42 //@ analysis(true)
	}
}

func fanin(ch1, ch2 <-chan int) <-chan int {
	c := make(chan int)
	go func() {
		for {
			select { //@ analysis(true)
			case s := <-ch1:
				c <- s
			case s := <-ch2:
				c <- s
			}
		}
	}()
	return c
}

func main() {
	input1 := make(chan int)
	input2 := make(chan int)
	go work(input1)
	go work(input2)
	c := fanin(input1, input2)
	for {
		println(<-c) //@ analysis(true)
	}
}
