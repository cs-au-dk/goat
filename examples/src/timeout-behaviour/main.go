package main

// GoLive: replaced fmt.Println with println

import (
	//"fmt"
	"time"
)

func main() {
	done := make(chan struct{})
	ch := make(chan int)
	go func(ch chan int, done chan struct{}) {
		time.Sleep(1 * time.Second)
		ch <- 42 //@ analysis(true)
		println("Sent")
		done <- struct{}{}
	}(ch, done)
	select { //@ analysis(true)
	case v := <-ch:
		println("received value of", v)
	case <-time.After(1 * time.Second):
		println("Timeout: spawn goroutine to cleanup")
		println("value received after cleanup:", <-ch)
	case <-time.After(1 * time.Second):
		println("Timeout2: spawn goroutine to cleanup")
		println("value received after cleanup:", <-ch)
	}
	<-done
	println("All Done")
}
