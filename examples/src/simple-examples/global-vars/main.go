package main

var ch1 = make(chan int)

func main() {
	<-ch1
}
