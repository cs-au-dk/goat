package main

func main() {
	ch := make(chan int)
	done := make(chan interface{})

	// Sender
	go func() {
		ch <- 0 //@ analysis(true)
		println("Sent first message")
		ch <- 1
		println("Sent second message")
	}()

	// Contest
	go func() {
		x := <-ch //@ analysis(true)
		println("Goro got: ", x)
		done <- struct{}{}
	}()


	x := <-ch //@ analysis(true)
	println("Main got: ", x)

	<-done
}
