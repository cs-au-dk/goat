package main

// GoLive: replaced fmt.Println with println

import (
	//"fmt"
	"time"
)

var i int

func worker(id int, jobQueue <-chan int, done <-chan struct{}) {
	for {
		select { // @ analysis(true) // Requires modeling of closed channels
		case jobID := <-jobQueue:
			println(id, "Executing job", jobID)
		case <-done:
			println(id, "Quits")
			return
		}
	}
}

func morejob() bool {
	i++
	return i < 20
}

func producer(q chan int, done chan struct{}) {
	// GoLive: unrolled loop a couple of times
	/*
	for morejob() {
		q <- i
	}
	*/
	q <- i
	i++
	q <- i
	i++
	q <- i
	close(done)
}

func main() {
	jobQueue := make(chan int)
	done := make(chan struct{})
	go worker(1, jobQueue, done)
	go worker(2, jobQueue, done)
	producer(jobQueue, done)
	time.Sleep(1 * time.Second)
}
