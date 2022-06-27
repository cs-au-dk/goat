package main

func f(ch chan int) {
	// This Goroutine will be encountered twice
	go func() {
		ch <- 10
	}()
}

func main() {
	ch := make(chan int)

	go func() {
		ch <- 10
	}()

	go func() {
		<-ch
		f(ch)
		f(ch)
	}()

	<-ch //@ analysis(true)
}
