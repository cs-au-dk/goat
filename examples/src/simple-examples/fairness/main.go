package main

func main() {
	ch := make(chan int)

	go func() {
		for {
			<-ch //@ analyze(true)
		}
	}()

	go func() {
		for {
			ch <- 10 //@ analyze(true)
		}
	}()


	<-ch //@ analyze(true)
}
