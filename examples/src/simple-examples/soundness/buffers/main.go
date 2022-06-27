package main

func main() {
	ch1 := make(chan int)
	ch2 := make(chan int, 1)

	go func() {
		ch1 <- 10 //@ analysis(true)
	}()

	go func() {
		ch2 <- 10
		<-ch1
	}()

	<-ch1 //@ analysis(false)
}
