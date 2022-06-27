package main

func main() {
	ch := make(chan int)
	closed := make(chan int)

	go func() {
		close(closed)
		ch <- 10
	}()

	go func() {
		<-closed //@ analysis(true)
		<-ch
	}()

	<-ch //@ analysis(false)
}
