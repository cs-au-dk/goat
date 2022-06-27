package main

// philio test case from Stadtmuller, Thieman

// GoLive: replaced fmt.Printf with println
// The example is also quite interesting as the first receive in each
// philo invocation is live, but the second might not be (if all three
// philosophers pick up a fork simultaneously)

import (
	//"fmt"
)

func philo(id int, forks chan int) {
	for {
		<-forks //@ analysis(true)
		<-forks
		println(id, "eats")
		forks <- 1
		forks <- 1
	}
}

func main() {
	forks := make(chan int)
	go func() { forks <- 1 }() //@ analysis(true)
	go func() { forks <- 1 }() //@ analysis(true)
	go func() { forks <- 1 }() //@ analysis(true)
	go philo(1, forks)
	go philo(2, forks)
	philo(3, forks)
}
