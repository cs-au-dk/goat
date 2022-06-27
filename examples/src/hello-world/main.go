package main

func main() {
	x := make(chan int)
	go func() {
		<-x //@ analysis(true)
	}()
	x <- 4 //@ analysis(true)
}
