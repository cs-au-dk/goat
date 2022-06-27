package main

func main() {
	go func() {
		a := make(chan int)
		go func() {
			a <- 10
		}()
		<-a
	}()
	go func() {
		b := make(chan int)
		go func() {
			b <- 10
		}()
		<-b
	}()
}
