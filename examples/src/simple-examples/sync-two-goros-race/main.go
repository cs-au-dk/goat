package main

func main() {
	ch := make(chan int)
	go func() {
		ch <- 10 //@ analysis(false)
	}()

	go func() {
		ch <- 20 //@ analysis(false)
	}()

	<-ch //@ analysis(true)
}
