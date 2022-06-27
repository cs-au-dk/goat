package main

func main() {
	a := make(chan int)
	go func() {
		b := make(chan int)
		go func() {
			b <- 10
		}()
	}()
	go func() {
		a <- 20
	}()
	<-a
}
