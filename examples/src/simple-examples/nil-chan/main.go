package main

func main() {
	var ch chan int
	go func() {
		ch <- 10
	}()
	if !(func() bool { return true }()) {
		ch = make(chan int)
	}
	<-ch
}
