package main

func main() {
	ch := make(chan int)
	go func() {
		ch <- 10 //@ analyze(true, 1)
	}()
	<-ch //@ analyze(true, 1)
}
