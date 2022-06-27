package main

// Producer-Consumer example.
// http://www.golangpatterns.info/concurrency/producer-consumer

// GoLive: replaced fmt.Println with println

//import "fmt"

var done = make(chan bool)
var msgs = make(chan int)

func produce() {
	for i := 0; i < 10; i++ {
		msgs <- i
	}
	done <- true
}

func consume() {
	for {
		msg := <-msgs
		println(msg)
	}
}

func main() {
	go produce()
	go consume()
	<-done // @ analysis(true) // enable when we can refine C and we have implemented assumption that loops terminate
}
