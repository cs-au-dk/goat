package main

func main() {
	ch := make(chan int)
	go func() { ch <- 10 }()
	go func() { ch <- 20 }()
	go func() {
		<-ch //@ analysis(true)
	}()

	<-ch //@ analysis(true)
}
