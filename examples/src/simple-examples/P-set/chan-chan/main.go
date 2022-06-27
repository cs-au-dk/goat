package main

func main() {
	x := make(chan int)
	ch := make(chan chan int)

	go func() {
		x := <-ch
		x <- 10
	}()

	ch <- x
	println(<-x)
	<-x
}
